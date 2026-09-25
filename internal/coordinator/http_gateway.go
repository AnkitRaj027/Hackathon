package coordinator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"vault/internal/erasure"
	"vault/internal/health"
	"vault/internal/placement"
	"vault/internal/repair"
	"vault/internal/scrubber"
	pbMeta "vault/proto/metadata"
)

// StorageEvent represents a real-time event broadcast to the 3D operations console.
type StorageEvent struct {
	Type      string      `json:"type"`      // "NODE_STATE_CHANGED", "CHUNK_REPLICATED", "REPAIR_STARTED", "REPAIR_COMPLETED", "CHECKSUM_FAILURE", "OBJECT_STORED", "OBJECT_DELETED", "NETWORK_PARTITION", "NETWORK_HEALED", "NODE_JOINED", "TOPOLOGY_CHANGED", "SYSTEM_LOG"
	Timestamp string      `json:"timestamp"` // ISO8601
	Payload   interface{} `json:"payload"`
}

// DriveInfo represents telemetry for an individual physical/virtual NVMe drive bay in a 2U chassis.
type DriveInfo struct {
	BayIndex    int      `json:"bay_index"`
	Slot        string   `json:"slot"`
	Status      string   `json:"status"` // "HEALTHY", "WARNING", "FAILED"
	Model       string   `json:"model"`
	CapacityGB  int      `json:"capacity_gb"`
	UsedGB      float64  `json:"used_gb"`
	Temperature int      `json:"temperature_c"`
	WearPct     int      `json:"wear_pct"`
	ChunksCount int      `json:"chunks_count"`
	ChunkIDs    []string `json:"chunk_ids"`
}

// MetricsPoint represents an aggregated cluster throughput measurement sample.
type MetricsPoint struct {
	Timestamp  string  `json:"timestamp"`
	WriteMBps  float64 `json:"write_mbps"`
	ReadMBps   float64 `json:"read_mbps"`
	IOPS       int     `json:"iops"`
	AvgLatency float64 `json:"avg_latency_ms"`
}

// HTTPGateway exposes REST, S3 API, and SSE telemetry endpoints and serves the 3D console.
type HTTPGateway struct {
	coord       *Service
	pool        *ClientPool
	ec          *erasure.Pipeline
	ring        placement.PlacementStrategy
	repairMgr   *repair.Manager
	subscribers map[chan StorageEvent]struct{}
	subMu       sync.RWMutex
	events      []StorageEvent
	eventsMu    sync.RWMutex

	// Real-time rolling metrics
	metricsMu   sync.RWMutex
	metricsHist []MetricsPoint
	writeBytes  int64
	readBytes   int64
	opsCount    int64
	latencySum  int64 // microseconds
}

// NewHTTPGateway creates a new HTTPGateway instance.
func NewHTTPGateway(coord *Service, pool *ClientPool) *HTTPGateway {
	gw := &HTTPGateway{
		coord:       coord,
		pool:        pool,
		subscribers: make(map[chan StorageEvent]struct{}),
		events:      make([]StorageEvent, 0),
		metricsHist: make([]MetricsPoint, 0),
	}

	go gw.startMetricsSampler()
	return gw
}

func (g *HTTPGateway) startMetricsSampler() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		wb := atomic.SwapInt64(&g.writeBytes, 0)
		rb := atomic.SwapInt64(&g.readBytes, 0)
		ops := atomic.SwapInt64(&g.opsCount, 0)
		latUs := atomic.SwapInt64(&g.latencySum, 0)

		avgLat := 0.0
		if ops > 0 {
			avgLat = float64(latUs) / float64(ops) / 1000.0 // ms
		}

		point := MetricsPoint{
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			WriteMBps:  float64(wb) / (1024 * 1024),
			ReadMBps:   float64(rb) / (1024 * 1024),
			IOPS:       int(ops),
			AvgLatency: avgLat,
		}

		g.metricsMu.Lock()
		g.metricsHist = append(g.metricsHist, point)
		if len(g.metricsHist) > 30 {
			g.metricsHist = g.metricsHist[len(g.metricsHist)-30:]
		}
		g.metricsMu.Unlock()
	}
}

// SetRing attaches the consistent hash ring strategy.
func (g *HTTPGateway) SetRing(ring placement.PlacementStrategy) {
	g.ring = ring
}

// SetRepairManager attaches the automated background repair manager.
func (g *HTTPGateway) SetRepairManager(rm *repair.Manager) {
	g.repairMgr = rm
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
	if len(g.events) > 200 {
		g.events = g.events[len(g.events)-200:]
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

// Handler returns the http.Handler with all REST, S3, and web console routes.
func (g *HTTPGateway) Handler() http.Handler {
	mux := http.NewServeMux()

	cors := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, HEAD, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, ETag, x-amz-date, x-amz-content-sha256")
			w.Header().Set("Access-Control-Expose-Headers", "ETag, Content-Length, Content-Type")
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
	mux.HandleFunc("/api/metrics", cors(g.handleMetrics))

	// Data Management
	mux.HandleFunc("/api/objects", cors(g.handleObjects))
	mux.HandleFunc("/api/objects/", cors(g.handleObjectByKey))
	mux.HandleFunc("/api/upload", cors(g.handleUpload))
	mux.HandleFunc("/api/download/", cors(g.handleDownload))

	// Chaos Controls (Real administrative operations)
	mux.HandleFunc("/api/admin/nodes/join", cors(g.handleAdminJoinNode))
	mux.HandleFunc("/api/admin/nodes/", cors(g.handleAdminNodeAction))
	mux.HandleFunc("/api/admin/chunks/", cors(g.handleAdminChunkAction))

	// S3-Compatible API Gateway
	mux.HandleFunc("/s3", cors(g.handleS3Root))
	mux.HandleFunc("/s3/", cors(g.handleS3Request))

	// Serve Static Frontend Assets (if built in frontend/dist)
	distDir := "frontend/dist"
	if _, err := os.Stat(distDir); err == nil {
		fs := http.FileServer(http.Dir(distDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/s3") || r.URL.Path == "/health" {
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
		close(ch)
		g.subMu.Unlock()
	}()

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
		if !g.pool.IsPartitioned(nid) {
			if _, err := g.pool.CheckStorageNode(ctx, nid); err == nil {
				healthyCount++
			}
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

func (g *HTTPGateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	g.metricsMu.RLock()
	res := make([]MetricsPoint, len(g.metricsHist))
	copy(res, g.metricsHist)
	g.metricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
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

		isPart := g.pool.IsPartitioned(id)
		if isPart {
			state = "PARTITIONED"
		} else {
			start := time.Now()
			_, err := g.pool.CheckStorageNode(ctx, id)
			if err != nil {
				state = "DEAD"
			} else {
				rttMs = float64(time.Since(start).Microseconds()) / 1000.0
			}
		}

		// Generate 12-bay chassis physical drive telemetry
		drives := make([]DriveInfo, 12)
		for bay := 0; bay < 12; bay++ {
			driveStatus := "HEALTHY"
			if state == "DEAD" || isPart {
				driveStatus = "FAILED"
			}
			drives[bay] = DriveInfo{
				BayIndex:    bay,
				Slot:        fmt.Sprintf("Bay %02d", bay),
				Status:      driveStatus,
				Model:       "Enterprise NVMe U.2 3.84TB",
				CapacityGB:  3840,
				UsedGB:      float64(bay*120 + 350),
				Temperature: 34 + (bay % 4),
				WearPct:     99 - (bay % 3),
				ChunksCount: 14 + bay*3,
				ChunkIDs:    []string{fmt.Sprintf("chk-%s-b%d-01", id, bay), fmt.Sprintf("chk-%s-b%d-02", id, bay)},
			}
		}

		rack := "rack-01"
		zone := "us-east-1a"
		if id == "storage-03" || id == "storage-04" {
			rack = "rack-02"
			zone = "us-east-1b"
		}

		result = append(result, map[string]interface{}{
			"id":             id,
			"address":        addr,
			"status":         state,
			"is_partitioned": isPart,
			"rtt_ms":         rttMs,
			"role":           "storage-engine",
			"rack":           rack,
			"zone":           zone,
			"drives":         drives,
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
		"ring_type":       "consistent_hash",
		"vnodes_per_node": vnodes,
		"physical_nodes":  nodeList,
		"total_points":    len(points),
		"ring_points":     points,
	})
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

	start := time.Now()
	err := r.ParseMultipartForm(128 * 1024 * 1024)
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
						"sha256":   ch.GetSha256(),
					},
				})
			}
		}
	}

	atomic.AddInt64(&g.writeBytes, int64(len(data)))
	atomic.AddInt64(&g.opsCount, 1)
	atomic.AddInt64(&g.latencySum, time.Since(start).Microseconds())

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
	_ = json.NewEncoder(w).Encode(createdObj)
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

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var data []byte
	var err error

	if strings.HasPrefix(key, "ec_") && g.ec != nil {
		data, err = g.ec.GetObjectEC(ctx, key)
		if err != nil {
			http.Error(w, "erasure download failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		data, err = g.coord.GetData(ctx, key)
		if err != nil {
			http.Error(w, "download failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	atomic.AddInt64(&g.readBytes, int64(len(data)))
	atomic.AddInt64(&g.opsCount, 1)
	atomic.AddInt64(&g.latencySum, time.Since(start).Microseconds())

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(key)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	_, _ = w.Write(data)
}

// handleAdminJoinNode dynamically registers and joins a new storage node.
func (g *HTTPGateway) handleAdminJoinNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID  string `json:"node_id"`
		Address string `json:"address"`
		Rack    string `json:"rack"`
		Zone    string `json:"zone"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.NodeID == "" {
		req.NodeID = "storage-04"
	}
	if req.Address == "" {
		req.Address = "127.0.0.1:50054"
	}
	if req.Rack == "" {
		req.Rack = "rack-02"
	}
	if req.Zone == "" {
		req.Zone = "us-east-1b"
	}

	g.pool.AddStorageNode(req.NodeID, req.Address)
	if chr, ok := g.ring.(*placement.ConsistentHashRing); ok {
		chr.AddNode(req.NodeID)
	}

	g.Broadcast(StorageEvent{
		Type:      "NODE_JOINED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"node_id": req.NodeID,
			"address": req.Address,
			"rack":    req.Rack,
			"zone":    req.Zone,
		},
	})

	g.Broadcast(StorageEvent{
		Type:      "TOPOLOGY_CHANGED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"reason":  fmt.Sprintf("Node %s joined consistent hash ring (rebalanced 256 virtual nodes)", req.NodeID),
			"node_id": req.NodeID,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "joined",
		"node_id": req.NodeID,
		"address": req.Address,
	})
}

// handleAdminNodeAction processes real chaos operations (kill, partition, heal, scrub).
func (g *HTTPGateway) handleAdminNodeAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/admin/nodes/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "expected /api/admin/nodes/{id}/{action}", http.StatusBadRequest)
		return
	}

	nodeID := parts[0]
	action := parts[1]

	switch action {
	case "kill":
		ports := map[string]int{
			"storage-01": 8081,
			"storage-02": 8082,
			"storage-03": 8083,
			"storage-04": 8084,
		}
		port, exists := ports[nodeID]
		if !exists {
			http.Error(w, fmt.Sprintf("unknown node %s", nodeID), http.StatusNotFound)
			return
		}

		client := &http.Client{Timeout: 2 * time.Second}
		_, _ = client.Post(fmt.Sprintf("http://127.0.0.1:%d/admin/kill", port), "application/json", nil)
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

	case "partition":
		g.pool.SetPartitioned(nodeID, true)
		g.Broadcast(StorageEvent{
			Type:      "NETWORK_PARTITION",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Payload: map[string]interface{}{
				"node_id": nodeID,
				"status":  "PARTITIONED",
				"reason":  fmt.Sprintf("Simulated network partition isolated node %s from coordinator and peers", nodeID),
			},
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "partitioned",
			"node_id": nodeID,
		})

	case "heal":
		g.pool.SetPartitioned(nodeID, false)
		g.Broadcast(StorageEvent{
			Type:      "NETWORK_HEALED",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Payload: map[string]interface{}{
				"node_id": nodeID,
				"status":  "HEALTHY",
				"reason":  fmt.Sprintf("Network partition healed for node %s", nodeID),
			},
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "healed",
			"node_id": nodeID,
		})

	case "scrub":
		scrub := scrubber.NewDiskScrubber(nodeID, fmt.Sprintf("data/%s", nodeID))
		report, err := scrub.Scrub(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("scrub failed: %v", err), http.StatusInternalServerError)
			return
		}

		g.Broadcast(StorageEvent{
			Type:      "SYSTEM_LOG",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Payload: map[string]interface{}{
				"node_id":          nodeID,
				"message":          fmt.Sprintf("Scrub completed: %d total, %d healthy, %d corrupted", report.TotalChunksScanned, report.HealthyChunks, len(report.CorruptedChunks)),
				"corrupted_chunks": len(report.CorruptedChunks),
			},
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(report)

	default:
		http.Error(w, fmt.Sprintf("unsupported action %s", action), http.StatusBadRequest)
	}
}

// handleAdminChunkAction injects physical bit-rot corruption and triggers automated self-healing.
func (g *HTTPGateway) handleAdminChunkAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/admin/chunks/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "corrupt" {
		http.Error(w, "expected /api/admin/chunks/{id}/corrupt", http.StatusBadRequest)
		return
	}

	chunkID := parts[0]
	corrupted := false
	var corruptedNode string

	dataDirs := []string{
		"data/storage-01",
		"data/storage-02",
		"data/storage-03",
		"data/storage-04",
	}

	for _, dir := range dataDirs {
		chunkPath := filepath.Join(dir, chunkID+".chunk")
		if _, err := os.Stat(chunkPath); err == nil {
			f, err := os.OpenFile(chunkPath, os.O_WRONLY, 0644)
			if err == nil {
				_, _ = f.WriteAt([]byte("CORRUPTED_BITROT_PAYLOAD_CHECKSUM_FAILURE"), 0)
				_ = f.Sync()
				_ = f.Close()
				corrupted = true
				corruptedNode = filepath.Base(dir)
			}
		}
	}

	if !corrupted {
		http.Error(w, fmt.Sprintf("chunk file %s not found on storage volumes", chunkID), http.StatusNotFound)
		return
	}

	// 1. Broadcast immediate checksum failure (visual turn red)
	g.Broadcast(StorageEvent{
		Type:      "CHECKSUM_FAILURE",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"chunk_id": chunkID,
			"node_id":  corruptedNode,
			"status":   "CORRUPTED",
			"message":  "Physical bit-rot injected into chunk block. SHA-256 integrity mismatch.",
		},
	})

	// 2. Trigger automated background repair sweep after short delay so operator observes failure
	if g.repairMgr != nil {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			_, _ = g.repairMgr.RunRepairCycle(context.Background())
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "corrupted",
		"chunk_id": chunkID,
		"node_id":  corruptedNode,
	})
}

// S3-Compatible API Handlers
func (g *HTTPGateway) handleS3Root(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	keys := make([]string, 0)
	for _, o := range resp.GetObjects() {
		keys = append(keys, o.GetKey())
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"bucket":  "vault",
		"objects": keys,
		"count":   len(keys),
	})
}

func (g *HTTPGateway) handleS3Request(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/s3/")
	parts := strings.SplitN(trimmed, "/", 2)
	bucket := parts[0]
	key := ""
	if len(parts) == 2 {
		key = parts[1]
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	start := time.Now()

	switch r.Method {
	case http.MethodGet:
		if key == "" {
			g.handleS3Root(w, r)
			return
		}
		data, err := g.coord.GetData(ctx, key)
		if err != nil {
			http.Error(w, "NoSuchKey", http.StatusNotFound)
			return
		}

		hash := sha256.Sum256(data)
		etag := fmt.Sprintf("\"%x\"", hash)

		atomic.AddInt64(&g.readBytes, int64(len(data)))
		atomic.AddInt64(&g.opsCount, 1)
		atomic.AddInt64(&g.latencySum, time.Since(start).Microseconds())

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		_, _ = w.Write(data)

	case http.MethodHead:
		if key == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		data, err := g.coord.GetData(ctx, key)
		if err != nil {
			http.Error(w, "NoSuchKey", http.StatusNotFound)
			return
		}
		hash := sha256.Sum256(data)
		w.Header().Set("ETag", fmt.Sprintf("\"%x\"", hash))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)

	case http.MethodPut:
		if key == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "IncompleteBody", http.StatusBadRequest)
			return
		}

		createdObj, err := g.coord.PutData(ctx, key, data)
		if err != nil {
			http.Error(w, fmt.Sprintf("InternalError: %v", err), http.StatusInternalServerError)
			return
		}

		hash := sha256.Sum256(data)
		etag := fmt.Sprintf("\"%x\"", hash)

		atomic.AddInt64(&g.writeBytes, int64(len(data)))
		atomic.AddInt64(&g.opsCount, 1)
		atomic.AddInt64(&g.latencySum, time.Since(start).Microseconds())

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
				"bucket": bucket,
				"size":   len(data),
				"s3":     true,
			},
		})

		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		if key == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		metaClient, err := g.pool.GetMetadataClient()
		if err == nil {
			_, _ = metaClient.DeleteObject(ctx, &pbMeta.DeleteObjectMetadataRequest{Key: key})
		}
		g.Broadcast(StorageEvent{
			Type:      "OBJECT_DELETED",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Payload: map[string]interface{}{
				"key":    key,
				"bucket": bucket,
				"s3":     true,
			},
		})
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "MethodNotAllowed", http.StatusMethodNotAllowed)
	}
}
