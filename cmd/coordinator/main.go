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
	"vault/internal/detector"
	"vault/internal/health"
	"vault/internal/logging"
	"vault/internal/placement"
	"vault/internal/repair"
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

	var placementStrategy placement.PlacementStrategy
	if cfg.PlacementStrategy == "fixed" {
		placementStrategy, err = placement.NewFixedReplicationPlacement(nodeIDs, cfg.ReplicationFactor)
	} else {
		// Phase 3: Consistent Hash Ring with 256 virtual nodes
		placementStrategy, err = placement.NewConsistentHashRing(nodeIDs, 256, cfg.ReplicationFactor)
	}
	if err != nil {
		logger.Error("failed initializing placement strategy", "error", err)
		os.Exit(1)
	}

	coordService := coordinator.NewService(cfg, pool, placementStrategy)

	// Phase 2: Active Failure Detector
	failDetector := detector.NewDetector(pool, nodeIDs, detector.DefaultConfig())
	failDetector.Start(context.Background())
	defer failDetector.Stop()
	coordService.SetDetector(failDetector)

	// Phase 2: Automated Background Self-Healing Repair Manager
	repairMgr := repair.NewManager(pool, failDetector, nodeIDs, cfg.ReplicationFactor, 5*time.Second)
	repairMgr.Start(context.Background())
	defer repairMgr.Stop()

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
