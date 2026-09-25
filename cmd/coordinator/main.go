package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"vault/internal/config"
	"vault/internal/coordinator"
	"vault/internal/health"
	"vault/internal/logging"
	"vault/internal/placement"
	pb "vault/proto/coordinator"
)

func main() {
	cfg, err := config.LoadCoordinatorConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	logger := logging.InitLogger("coordinator", "")
	logger.Info("starting coordinator service",
		"grpc_addr", cfg.ListenAddr,
		"http_port", cfg.HTTPPort,
		"metadata_addr", cfg.MetadataAddr,
		"chunk_size", cfg.ChunkSize,
		"rf", cfg.ReplicationFactor,
		"write_quorum", cfg.WriteQuorum,
		"read_quorum", cfg.ReadQuorum,
	)

	// Health check server
	healthServer := health.StartHealthServer(cfg.HTTPPort, "coordinator", "")

	pool := coordinator.NewClientPool(cfg.MetadataAddr, cfg.StorageNodes)
	defer pool.Close()

	var nodeIDs []string
	for id := range cfg.StorageNodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	placementStrategy, err := placement.NewFixedReplicationPlacement(nodeIDs, cfg.ReplicationFactor)
	if err != nil {
		logger.Error("failed initializing placement strategy", "error", err)
		os.Exit(1)
	}

	coordService := coordinator.NewService(cfg, pool, placementStrategy)

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		logger.Error("failed to listen on gRPC address", "addr", cfg.ListenAddr, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)
	pb.RegisterCoordinatorServiceServer(grpcServer, coordService)

	go func() {
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			logger.Error("gRPC server error", "error", err)
		}
	}()

	logger.Info("coordinator is ready and serving", "addr", lis.Addr().String())

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down coordinator gracefully...")
	grpcServer.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = healthServer.Shutdown(ctx)

	logger.Info("coordinator stopped")
}
