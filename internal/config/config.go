package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// StorageNodeConfig holds configuration for an individual storage node.
type StorageNodeConfig struct {
	NodeID     string
	DataDir    string
	ListenAddr string
	HTTPPort   int
}

// LoadStorageNodeConfig loads configuration from environment variables with defaults.
func LoadStorageNodeConfig() (*StorageNodeConfig, error) {
	nodeID := getEnv("VAULT_NODE_ID", "storage-01")
	dataDir := getEnv("VAULT_DATA_DIR", "./data/"+nodeID)
	listenAddr := getEnv("VAULT_GRPC_ADDR", ":50051")
	httpPort, _ := strconv.Atoi(getEnv("VAULT_HTTP_PORT", "8081"))

	if nodeID == "" {
		return nil, fmt.Errorf("VAULT_NODE_ID must be set")
	}

	return &StorageNodeConfig{
		NodeID:     nodeID,
		DataDir:    dataDir,
		ListenAddr: listenAddr,
		HTTPPort:   httpPort,
	}, nil
}

// MetadataConfig holds configuration for the Metadata service.
type MetadataConfig struct {
	ListenAddr    string
	HTTPPort      int
	EtcdEndpoints []string
}

// LoadMetadataConfig loads metadata service configuration.
func LoadMetadataConfig() (*MetadataConfig, error) {
	listenAddr := getEnv("VAULT_METADATA_GRPC_ADDR", ":50050")
	httpPort, _ := strconv.Atoi(getEnv("VAULT_METADATA_HTTP_PORT", "8080"))
	rawEndpoints := getEnv("VAULT_ETCD_ENDPOINTS", "localhost:2379")

	endpoints := strings.Split(rawEndpoints, ",")
	for i := range endpoints {
		endpoints[i] = strings.TrimSpace(endpoints[i])
	}

	return &MetadataConfig{
		ListenAddr:    listenAddr,
		HTTPPort:      httpPort,
		EtcdEndpoints: endpoints,
	}, nil
}

// CoordinatorConfig holds configuration for the Coordinator gateway service.
type CoordinatorConfig struct {
	ListenAddr        string
	HTTPPort          int
	MetadataAddr      string
	StorageNodes      map[string]string // nodeID -> address
	ChunkSize         int64
	ReplicationFactor int
	WriteQuorum       int
	ReadQuorum        int
	PlacementStrategy string
}

// LoadCoordinatorConfig loads coordinator configuration.
func LoadCoordinatorConfig() (*CoordinatorConfig, error) {
	listenAddr := getEnv("VAULT_COORDINATOR_GRPC_ADDR", ":50055")
	httpPort, _ := strconv.Atoi(getEnv("VAULT_COORDINATOR_HTTP_PORT", "8085"))
	metadataAddr := getEnv("VAULT_METADATA_ADDR", "localhost:50050")
	placementStrategy := getEnv("VAULT_PLACEMENT_STRATEGY", "consistent_hash")

	chunkSize, err := strconv.ParseInt(getEnv("VAULT_CHUNK_SIZE", "4194304"), 10, 64)
	if err != nil || chunkSize <= 0 {
		chunkSize = 4 * 1024 * 1024 // 4 MiB
	}

	rf, err := strconv.Atoi(getEnv("VAULT_REPLICATION_FACTOR", "3"))
	if err != nil || rf <= 0 {
		rf = 3
	}

	wq, err := strconv.Atoi(getEnv("VAULT_WRITE_QUORUM", "3"))
	if err != nil || wq <= 0 {
		wq = 3
	}

	rq, err := strconv.Atoi(getEnv("VAULT_READ_QUORUM", "1"))
	if err != nil || rq <= 0 {
		rq = 1
	}

	rawNodes := getEnv("VAULT_STORAGE_NODES", "storage-01=localhost:50051,storage-02=localhost:50052,storage-03=localhost:50053")
	storageNodes := make(map[string]string)
	for _, pair := range strings.Split(rawNodes, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			storageNodes[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return &CoordinatorConfig{
		ListenAddr:        listenAddr,
		HTTPPort:          httpPort,
		MetadataAddr:      metadataAddr,
		StorageNodes:      storageNodes,
		ChunkSize:         chunkSize,
		ReplicationFactor: rf,
		WriteQuorum:       wq,
		ReadQuorum:        rq,
		PlacementStrategy: placementStrategy,
	}, nil
}

// ClientConfig holds configuration for the vaultctl CLI.
type ClientConfig struct {
	CoordinatorAddr string
}

// LoadClientConfig loads CLI configuration.
func LoadClientConfig() *ClientConfig {
	return &ClientConfig{
		CoordinatorAddr: getEnv("VAULT_COORDINATOR_ADDR", "localhost:50055"),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}
