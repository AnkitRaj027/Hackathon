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
	"vault/internal/metadata"
	pb "vault/proto/metadata"
)

func main() {
	cfg, err := config.LoadMetadataConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	logger := logging.InitLogger("metadata-service", "")
	logger.Info("starting metadata service",
		"grpc_addr", cfg.ListenAddr,
		"http_port", cfg.HTTPPort,
		"etcd_endpoints", cfg.EtcdEndpoints,
	)

	// Health check server
	healthServer := health.StartHealthServer(cfg.HTTPPort, "metadata", "")

	// Initialize Store: try etcd first, fallback to memory if explicitly configured or etcd unreachable in dev
	var store metadata.Store
	etcdStore, err := metadata.NewEtcdStore(cfg.EtcdEndpoints)
	if err != nil {
		logger.Warn("failed connecting to etcd, falling back to in-memory store for local mode", "error", err)
		store = metadata.NewMemoryStore()
	} else {
		logger.Info("connected to etcd consensus backend successfully", "endpoints", cfg.EtcdEndpoints)
		store = etcdStore
	}
	defer store.Close()

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		logger.Error("failed to listen on gRPC address", "addr", cfg.ListenAddr, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)
	pb.RegisterMetadataServiceServer(grpcServer, metadata.NewServer(store))

	go func() {
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			logger.Error("gRPC server error", "error", err)
		}
	}()

	logger.Info("metadata service is ready and serving", "addr", lis.Addr().String())

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down metadata service gracefully...")
	grpcServer.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = healthServer.Shutdown(ctx)

	logger.Info("metadata service stopped")
}
