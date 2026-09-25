package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"vault/internal/config"
	"vault/internal/health"
	"vault/internal/logging"
	"vault/internal/scrubber"
	"vault/internal/storage"
	pb "vault/proto/storage"
)

func main() {
	cfg, err := config.LoadStorageNodeConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	logger := logging.InitLogger("storage-node", cfg.NodeID)
	logger.Info("starting storage node",
		"node_id", cfg.NodeID,
		"grpc_addr", cfg.ListenAddr,
		"data_dir", cfg.DataDir,
		"http_port", cfg.HTTPPort,
	)

	// Health check server
	healthServer := health.StartHealthServer(cfg.HTTPPort, "storage", cfg.NodeID)

	storageServer, err := storage.NewServer(cfg.NodeID, cfg.DataDir)
	if err != nil {
		logger.Error("failed initializing storage server", "error", err)
		os.Exit(1)
	}

	// Phase 2: Background Disk Scrubber (Bit-Rot Detection)
	diskScrubber := scrubber.NewDiskScrubber(cfg.NodeID, cfg.DataDir)
	periodicScrubber := scrubber.NewPeriodicScrubber(diskScrubber, 30*time.Second, func(rep *scrubber.ScrubReport) {
		if len(rep.CorruptedChunks) > 0 {
			logger.Error("scrubber_detected_bit_rot", "corrupted_count", len(rep.CorruptedChunks), "node", cfg.NodeID)
		}
	})
	periodicScrubber.Start(context.Background())
	defer periodicScrubber.Stop()

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		logger.Error("failed to listen on gRPC address", "addr", cfg.ListenAddr, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)
	pb.RegisterStorageServiceServer(grpcServer, storageServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			logger.Error("gRPC server error", "error", err)
		}
	}()

	logger.Info("storage node is ready and serving", "node_id", cfg.NodeID, "addr", lis.Addr().String())

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down storage node gracefully...")
	grpcServer.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = healthServer.Shutdown(ctx)

	logger.Info("storage node stopped")
}
