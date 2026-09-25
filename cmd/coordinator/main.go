package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"vault/internal/config"
	"vault/internal/coordinator"
	"vault/internal/detector"
	"vault/internal/erasure"
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
	logger.Info("starting coordinator",
		"grpc_addr", cfg.ListenAddr,
		"http_port", cfg.HTTPPort,
		"metadata_addr", cfg.MetadataAddr,
		"chunk_size", cfg.ChunkSize,
		"rf", cfg.ReplicationFactor,
		"write_quorum", cfg.WriteQuorum,
		"read_quorum", cfg.ReadQuorum,
	)

	pool := coordinator.NewClientPool(cfg.MetadataAddr, cfg.StorageNodes)
	defer pool.Close()

	nodeIDs := make([]string, 0, len(cfg.StorageNodes))
	for nid := range cfg.StorageNodes {
		nodeIDs = append(nodeIDs, nid)
	}
	sort.Strings(nodeIDs)

	// Phase 3: Consistent Hash Ring with 256 virtual nodes per storage node
	var placementStrategy placement.PlacementStrategy
	ring, ringErr := placement.NewConsistentHashRing(nodeIDs, 256, cfg.ReplicationFactor)
	if ringErr != nil {
		logger.Warn("failed to initialize consistent hash ring, falling back to fixed replication", "error", ringErr)
		fallback, _ := placement.NewFixedReplicationPlacement(nodeIDs, cfg.ReplicationFactor)
		placementStrategy = fallback
	} else {
		placementStrategy = ring
		logger.Info("consistent hash ring initialized", "nodes", len(nodeIDs), "vnodes_per_node", 256, "rf", cfg.ReplicationFactor)
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

	// REST & 3D Operations Console Gateway
	httpGateway := coordinator.NewHTTPGateway(coordService, pool)
	httpGateway.SetRing(placementStrategy)
	httpGateway.SetRepairManager(repairMgr)

	if ring != nil {
		if ecPipeline, ecErr := erasure.NewPipeline(2, 1, pool, ring); ecErr == nil {
			httpGateway.SetErasurePipeline(ecPipeline)
		}
	}

	// Wire Failure Detector real-time state changes to console SSE stream
	failDetector.OnStatusChange(func(nodeID string, oldStatus, newStatus detector.NodeStatus) {
		httpGateway.Broadcast(coordinator.StorageEvent{
			Type:      "NODE_STATE_CHANGED",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Payload: map[string]interface{}{
				"node_id": nodeID,
				"status":  string(newStatus),
				"old":     string(oldStatus),
			},
		})
	})

	// Wire Repair Manager events to console SSE stream
	repairMgr.SetEventCallback(func(event string, chunkID string, srcNode string, targetNode string) {
		httpGateway.Broadcast(coordinator.StorageEvent{
			Type:      event,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Payload: map[string]interface{}{
				"chunk_id": chunkID,
				"source":   srcNode,
				"target":   targetNode,
			},
		})
	})

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: httpGateway.Handler(),
	}
	go func() {
		logger.Info("coordinator HTTP REST & Web Console is serving", "port", cfg.HTTPPort)
		_ = httpServer.ListenAndServe()
	}()

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
	_ = httpServer.Shutdown(ctx)

	logger.Info("coordinator stopped")
}
