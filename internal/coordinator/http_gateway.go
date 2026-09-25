package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vault/internal/erasure"
	"vault/internal/health"
	"vault/internal/placement"
	pbMeta "vault/proto/metadata"
)

// StorageEvent represents a real-time event broadcast to the 3D operations console.
type StorageEvent struct {
	Type      string      `json:"type"`      // "NODE_STATE_CHANGED", "CHUNK_REPLICATED", "REPAIR_STARTED", "REPAIR_COMPLETED", "CHECKSUM_FAILURE", "OBJECT_STORED", "OBJECT_DELETED", "SYSTEM_LOG"
	Timestamp string      `json:"timestamp"` // ISO8601
	Payload   interface{} `json:"payload"`
}

// HTTPGateway exposes REST and SSE telemetry endpoints and serves the 3D console.
type HTTPGateway struct {
	coord       *Service
	pool        *ClientPool
	ec          *erasure.Pipeline
	ring        placement.PlacementStrategy
	subscribers map[chan StorageEvent]struct{}
	subMu       sync.RWMutex
	events      []StorageEvent
	eventsMu    sync.RWMutex
}

// NewHTTPGateway creates a new HTTPGateway instance.
func NewHTTPGateway(coord *Service, pool *ClientPool) *HTTPGateway {
	return &HTTPGateway{
		coord:       coord,
		pool:        pool,
		subscribers: make(map[chan StorageEvent]struct{}),
		events:      make([]StorageEvent, 0),
	}
}

// SetRing attaches the consistent hash ring strategy.
func (g *HTTPGateway) SetRing(ring placement.PlacementStrategy) {
	g.ring = ring
}

// SetErasurePipeline attaches the Reed-Solomon pipeline to the gateway.
func (g *HTTPGateway) SetErasurePipeline(ec *erasure.Pipeline) {
	g.ec = ec
}

// Broadcast distributes an event to all connected SSE clients and retains it in history.
func (g *HTTPGateway) Broadcast(event StorageEvent) {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	g.eventsMu.Lock()
	g.events = append(g.events, event)
	if len(g.events) > 100 {
		g.events = g.events[len(g.events)-100:]
	}
	g.eventsMu.Unlock()

	g.subMu.RLock()
	defer g.subMu.RUnlock()
	for ch := range g.subscribers {
		select {
		case ch <- event:
		default:
			// Subscriber buffer full; skip to maintain gateway throughput
		}
	}
}

// Handler returns the http.Handler with all REST and web console routes.
func (g *HTTPGateway) Handler() http.Handler {
	mux := http.NewServeMux()

	cors := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next(w, r)
		}
	}

	// Health check
	mux.HandleFunc("/health", cors(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(health.Response{
			Status:  "healthy",
			Service: "coordinator",
		})
	}))

	// Telemetry & Status
	mux.HandleFunc("/api/status", cors(g.handleStatus))
	mux.HandleFunc("/api/nodes", cors(g.handleNodes))
	mux.HandleFunc("/api/topology", cors(g.handleTopology))
	mux.HandleFunc("/api/events", cors(g.handleEvents))

	// Data Management
	mux.HandleFunc("/api/objects", cors(g.handleObjects))
	mux.HandleFunc("/api/objects/", cors(g.handleObjectByKey))
	mux.HandleFunc("/api/upload", cors(g.handleUpload))
	mux.HandleFunc("/api/download/", cors(g.handleDownload))

	// Chaos Controls (Real administrative operations)
	mux.HandleFunc("/api/admin/nodes/", cors(g.handleAdminNodeAction))
	mux.HandleFunc("/api/admin/chunks/", cors(g.handleAdminChunkAction))

	// Serve Static Frontend Assets (if built in frontend/dist)
	distDir := "frontend/dist"
	if _, err := os.Stat(distDir); err == nil {
		fs := http.FileServer(http.Dir(distDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" {
				http.NotFound(w, r)
				return
			}
			path := filepath.Join(distDir, filepath.Clean("/"+r.URL.Path))
			if _, err := os.Stat(path); os.IsNotExist(err) && !strings.Contains(r.URL.Path, ".") {
				http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
	}

	return mux
}

// handleEvents streams real-time system events via Server-Sent Events (SSE).
func (g *HTTPGateway) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan StorageEvent, 50)

	g.subMu.Lock()
	g.subscribers[ch] = struct{}{}
	g.subMu.Unlock()

	defer func() {
		g.subMu.Lock()
		delete(g.subscribers, ch)
		g.subMu.Unlock()
	}()

	// Send recent events as initial replay
	g.eventsMu.RLock()
	recent := make([]StorageEvent, len(g.events))
	copy(recent, g.events)
	g.eventsMu.RUnlock()

	for _, ev := range recent {
		data, err := json.Marshal(ev)
		if err == nil {
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
	}
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case ev := <-ch:
			data, err := json.Marshal(ev)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

func (g *HTTPGateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	metaClient, err := g.pool.GetMetadataClient()
	if err != nil {
		http.Error(w, "metadata unreachable", http.StatusServiceUnavailable)
		return
	}

	resp, err := metaClient.ListObjects(ctx, &pbMeta.ListObjectsRequest{})
	activeObjs := 0
	if err == nil && resp != nil {
		activeObjs = len(resp.GetObjects())
	}

	nodes := g.pool.StorageNodes()
	healthyCount := 0
	for nid := range nodes {
		if _, err := g.pool.CheckStorageNode(ctx, nid); err == nil {
			healthyCount++
		}
	}

	hasEC := g.ec != nil
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":             "HEALTHY",
		"coordinator":        "127.0.0.1:50055",
		"total_nodes":        len(nodes),
		"healthy_nodes":      healthyCount,
		"active_objects":     activeObjs,
		"replication_factor": 3,
		"write_quorum":       3,
		"read_quorum":        1,
		"erasure_coding":     hasEC,
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
	})
}

func (g *HTTPGateway) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	nodes := g.pool.StorageNodes()
	result := make([]map[string]interface{}, 0, len(nodes))

	for id, addr := range nodes {
		state := "HEALTHY"
		rttMs := 0.0

		start := time.Now()
		_, err := g.pool.CheckStorageNode(ctx, id)
		if err != nil {
			state = "DEAD"
		} else {
			rttMs = float64(time.Since(start).Microseconds()) / 1000.0
		}

		result = append(result, map[string]interface{}{
			"id":       id,
			"address":  addr,
			"status":   state,
			"rtt_ms":   rttMs,
			"role":     "storage-engine",
			"rack":     "rack-01",
			"zone":     "us-east-1a",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (g *HTTPGateway) handleTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vnodes := 128
	var points []placement.RingPoint

	if chr, ok := g.ring.(*placement.ConsistentHashRing); ok {
		vnodes, points = chr.GetRingSnapshot()
	}

	nodes := g.pool.StorageNodes()
	nodeList := make([]string, 0, len(nodes))
	for n := range nodes {
		nodeList = append(nodeList, n)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ring_type":       "ConsistentHashRing-32bit",
		"vnodes_per_node": vnodes,
		"physical_nodes":  nodeList,
		"ring_points":     points,
		"total_points":    len(points),
	})
}

func (g *HTTPGateway) handleObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	metaClient, err := g.pool.GetMetadataClient()
	if err != nil {
		http.Error(w, "metadata unreachable", http.StatusServiceUnavailable)
		return
	}

	resp, err := metaClient.ListObjects(ctx, &pbMeta.ListObjectsRequest{})
	if err != nil {
		http.Error(w, "failed listing objects: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type chunkDTO struct {
		ChunkID  string   `json:"chunk_id"`
		Index    int64    `json:"index"`
		Size     int64    `json:"size"`
		Sha256   string   `json:"sha256"`
		Replicas []string `json:"replicas"`
		IsParity bool     `json:"is_parity"`
	}

	type objDTO struct {
		Key       string     `json:"key"`
		Size      int64      `json:"size"`
		Chunks    []chunkDTO `json:"chunks"`
		CreatedAt string     `json:"created_at"`
		Checksum  string     `json:"checksum"`
		Scheme    string     `json:"scheme"`
	}

	result := make([]objDTO, 0, len(resp.GetObjects()))
	for _, obj := range resp.GetObjects() {
		chunks := make([]chunkDTO, 0, len(obj.GetChunks()))
		hasParity := false
		for _, ch := range obj.GetChunks() {
			isP := strings.Contains(ch.GetChunkId(), ".parity.")
			if isP {
				hasParity = true
			}
			chunks = append(chunks, chunkDTO{
				ChunkID:  ch.GetChunkId(),
				Index:    ch.GetIndex(),
				Size:     ch.GetSize(),
				Sha256:   ch.GetSha256(),
				Replicas: ch.GetReplicas(),
				IsParity: isP,
			})
		}
		scheme := "3x-replication"
		if hasParity {
			scheme = "reed-solomon-2+1"
		}

		chk := ""
		if obj.GetCustomMetadata() != nil {
			chk = obj.GetCustomMetadata()["checksum"]
		}

		result = append(result, objDTO{
			Key:       obj.GetKey(),
			Size:      obj.GetSize(),
			Chunks:    chunks,
			CreatedAt: time.Unix(obj.GetCreatedAt(), 0).UTC().Format(time.RFC3339),
			Checksum:  chk,
			Scheme:    scheme,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (g *HTTPGateway) handleObjectByKey(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/objects/")
	if key == "" {
		http.Error(w, "missing object key", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	metaClient, err := g.pool.GetMetadataClient()
	if err != nil {
		http.Error(w, "metadata service unreachable", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		obj, err := metaClient.GetObject(ctx, &pbMeta.GetObjectMetadataRequest{Key: key})
		if err != nil {
			http.Error(w, "object not found: "+err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(obj.GetMetadata())

	case http.MethodDelete:
		_, err := metaClient.DeleteObject(ctx, &pbMeta.DeleteObjectMetadataRequest{Key: key})
		if err != nil {
			http.Error(w, "failed deleting object: "+err.Error(), http.StatusInternalServerError)
			return
		}
		g.Broadcast(StorageEvent{
			Type:      "OBJECT_DELETED",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Payload:   map[string]string{"key": key},
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "key": key})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (g *HTTPGateway) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(128 * 1024 * 1024) // 128 MB max memory
	if err != nil {
		http.Error(w, "failed parsing multipart body: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file in form data: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	key := r.FormValue("key")
	if key == "" {
		key = header.Filename
	}

	scheme := r.FormValue("scheme")
	if scheme == "" {
		scheme = "replication"
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed reading uploaded file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	var createdObj *pbMeta.ObjectMetadata
	if scheme == "erasure" && g.ec != nil {
		createdObj, err = g.ec.PutObjectEC(ctx, key, data)
		if err != nil {
			http.Error(w, "erasure coded put failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		createdObj, err = g.coord.PutData(ctx, key, data)
		if err != nil {
			http.Error(w, "replication put failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if createdObj != nil {
		for _, ch := range createdObj.GetChunks() {
			for _, rNode := range ch.GetReplicas() {
				g.Broadcast(StorageEvent{
					Type:      "CHUNK_REPLICATED",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Payload: map[string]interface{}{
						"chunk_id": ch.GetChunkId(),
						"target":   rNode,
						"size":     ch.GetSize(),
						"object":   key,
					},
				})
			}
		}
	}

	g.Broadcast(StorageEvent{
		Type:      "OBJECT_STORED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"key":    key,
			"size":   len(data),
			"scheme": scheme,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"key":    key,
		"size":   len(data),
		"scheme": scheme,
	})
}

func (g *HTTPGateway) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/api/download/")
	if key == "" {
		http.Error(w, "missing object key", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	metaClient, err := g.pool.GetMetadataClient()
	if err != nil {
		http.Error(w, "metadata unreachable", http.StatusServiceUnavailable)
		return
	}

	objMeta, err := metaClient.GetObject(ctx, &pbMeta.GetObjectMetadataRequest{Key: key})
	if err != nil {
		http.Error(w, "object not found: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(key)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", objMeta.GetMetadata().GetSize()))

	// If Erasure Coded (contains .ec.shard. or custom_metadata) use EC pipeline
	isEC := false
	if objMeta.GetMetadata().GetCustomMetadata() != nil && objMeta.GetMetadata().GetCustomMetadata()["encoding"] == "erasure_coding" {
		isEC = true
	}
	for _, c := range objMeta.GetMetadata().GetChunks() {
		if strings.Contains(c.GetChunkId(), ".ec.shard.") || strings.Contains(c.GetChunkId(), ".parity.") {
			isEC = true
			break
		}
	}

	var data []byte
	if isEC && g.ec != nil {
		data, err = g.ec.GetObjectEC(ctx, objMeta.GetMetadata())
		if err != nil {
			http.Error(w, "erasure decoding failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		data, err = g.coord.GetData(ctx, key)
		if err != nil {
			http.Error(w, "download failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	_, _ = w.Write(data)
}

// handleAdminNodeAction executes real destructive chaos operations on storage nodes.
func (g *HTTPGateway) handleAdminNodeAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Route format: /api/admin/nodes/{id}/kill
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/nodes/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "kill" {
		http.Error(w, "expected /api/admin/nodes/{id}/kill", http.StatusBadRequest)
		return
	}

	nodeID := parts[0]
	ports := map[string]int{
		"storage-01": 8081,
		"storage-02": 8082,
		"storage-03": 8083,
	}

	port, exists := ports[nodeID]
	if !exists {
		http.Error(w, fmt.Sprintf("unknown node %s", nodeID), http.StatusNotFound)
		return
	}

	// Trigger real process termination via storage node's HTTP server
	client := &http.Client{Timeout: 2 * time.Second}
	_, _ = client.Post(fmt.Sprintf("http://127.0.0.1:%d/admin/kill", port), "application/json", nil)

	// Sever connection pool
	g.pool.InvalidateStorageConn(nodeID)

	g.Broadcast(StorageEvent{
		Type:      "NODE_STATE_CHANGED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"node_id": nodeID,
			"status":  "DEAD",
			"reason":  "Operator administrative kill command executed",
		},
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "terminated",
		"node_id": nodeID,
	})
}

// handleAdminChunkAction injects physical bit-rot corruption into a chunk file on disk.
func (g *HTTPGateway) handleAdminChunkAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Route format: /api/admin/chunks/{id}/corrupt
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/chunks/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "corrupt" {
		http.Error(w, "expected /api/admin/chunks/{id}/corrupt", http.StatusBadRequest)
		return
	}

	chunkID := parts[0]
	corrupted := false

	// Locate chunk on any of the local storage volumes
	dataDirs := []string{
		"data/storage-01",
		"data/storage-02",
		"data/storage-03",
	}

	for _, dir := range dataDirs {
		chunkPath := filepath.Join(dir, chunkID+".chunk")
		if _, err := os.Stat(chunkPath); err == nil {
			f, err := os.OpenFile(chunkPath, os.O_WRONLY, 0644)
			if err == nil {
				_, _ = f.WriteAt([]byte("CORRUPTED_BITROT_PAYLOAD"), 0)
				_ = f.Sync()
				_ = f.Close()
				corrupted = true
			}
		}
	}

	if !corrupted {
		http.Error(w, fmt.Sprintf("chunk file %s not found on storage volumes", chunkID), http.StatusNotFound)
		return
	}

	g.Broadcast(StorageEvent{
		Type:      "CHECKSUM_FAILURE",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"chunk_id": chunkID,
			"message":  "Physical bit-rot injected into chunk payload on storage volume",
		},
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "corrupted",
		"chunk_id": chunkID,
	})
}
