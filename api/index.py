import os
import sys
import time
import asyncio
from pathlib import Path
from typing import Any

# Ensure project root and backend are on sys.path for Vercel serverless execution
CURRENT_DIR = Path(__file__).resolve().parent
ROOT_DIR = CURRENT_DIR.parent
if str(ROOT_DIR) not in sys.path:
    sys.path.insert(0, str(ROOT_DIR))

import httpx
from fastapi import APIRouter, File, Form, HTTPException, Request, Response, UploadFile
from fastapi.responses import JSONResponse, StreamingResponse
from backend.main import create_app
from backend.ai.config import VAULT_BACKEND_URL

app = create_app()

# Full node state with NVMe drive telemetry for cloud deployments
MOCK_NODES = [
    {
        "id": "node-1",
        "address": "10.0.1.11:50051",
        "status": "HEALTHY",
        "is_partitioned": False,
        "rtt_ms": 1.12,
        "role": "PRIMARY_STORAGE",
        "rack": "rack-alpha",
        "zone": "us-east-1a",
        "used_space": 42000000000,
        "total_space": 100000000000,
        "drives": [
            {"bay_index": 0, "slot": "bay-0", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1612, "temperature_c": 34, "wear_pct": 12, "chunks_count": 48, "chunk_ids": ["chk-weights-00", "chk-ledger-00"]},
            {"bay_index": 1, "slot": "bay-1", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1740, "temperature_c": 35, "wear_pct": 14, "chunks_count": 52, "chunk_ids": ["chk-weights-03"]},
            {"bay_index": 2, "slot": "bay-2", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1520, "temperature_c": 33, "wear_pct": 10, "chunks_count": 45, "chunk_ids": []},
            {"bay_index": 3, "slot": "bay-3", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1390, "temperature_c": 33, "wear_pct": 9,  "chunks_count": 39, "chunk_ids": []},
        ],
    },
    {
        "id": "node-2",
        "address": "10.0.1.12:50052",
        "status": "HEALTHY",
        "is_partitioned": False,
        "rtt_ms": 1.45,
        "role": "REPLICA_STORAGE",
        "rack": "rack-alpha",
        "zone": "us-east-1a",
        "used_space": 38000000000,
        "total_space": 100000000000,
        "drives": [
            {"bay_index": 0, "slot": "bay-0", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1420, "temperature_c": 32, "wear_pct": 8,  "chunks_count": 41, "chunk_ids": ["chk-weights-00", "chk-weights-01"]},
            {"bay_index": 1, "slot": "bay-1", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1680, "temperature_c": 34, "wear_pct": 11, "chunks_count": 46, "chunk_ids": ["chk-ledger-00", "chk-weights-03"]},
            {"bay_index": 2, "slot": "bay-2", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1810, "temperature_c": 35, "wear_pct": 15, "chunks_count": 50, "chunk_ids": []},
            {"bay_index": 3, "slot": "bay-3", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1350, "temperature_c": 32, "wear_pct": 7,  "chunks_count": 38, "chunk_ids": []},
        ],
    },
    {
        "id": "node-3",
        "address": "10.0.1.13:50053",
        "status": "HEALTHY",
        "is_partitioned": False,
        "rtt_ms": 1.28,
        "role": "REPLICA_STORAGE",
        "rack": "rack-beta",
        "zone": "us-east-1b",
        "used_space": 45000000000,
        "total_space": 100000000000,
        "drives": [
            {"bay_index": 0, "slot": "bay-0", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1550, "temperature_c": 33, "wear_pct": 10, "chunks_count": 44, "chunk_ids": ["chk-weights-00", "chk-weights-01", "chk-weights-02"]},
            {"bay_index": 1, "slot": "bay-1", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1620, "temperature_c": 34, "wear_pct": 12, "chunks_count": 47, "chunk_ids": ["chk-ledger-01"]},
            {"bay_index": 2, "slot": "bay-2", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1480, "temperature_c": 33, "wear_pct": 9,  "chunks_count": 42, "chunk_ids": []},
            {"bay_index": 3, "slot": "bay-3", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1290, "temperature_c": 31, "wear_pct": 6,  "chunks_count": 36, "chunk_ids": []},
        ],
    },
    {
        "id": "node-4",
        "address": "10.0.1.14:50054",
        "status": "HEALTHY",
        "is_partitioned": False,
        "rtt_ms": 1.34,
        "role": "PARITY_STORAGE",
        "rack": "rack-beta",
        "zone": "us-east-1b",
        "used_space": 31000000000,
        "total_space": 100000000000,
        "drives": [
            {"bay_index": 0, "slot": "bay-0", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1690, "temperature_c": 35, "wear_pct": 13, "chunks_count": 49, "chunk_ids": ["chk-weights-01", "chk-weights-02", "chk-weights-03"]},
            {"bay_index": 1, "slot": "bay-1", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1730, "temperature_c": 36, "wear_pct": 14, "chunks_count": 51, "chunk_ids": ["chk-ledger-00", "chk-ledger-01"]},
            {"bay_index": 2, "slot": "bay-2", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1510, "temperature_c": 33, "wear_pct": 11, "chunks_count": 43, "chunk_ids": []},
            {"bay_index": 3, "slot": "bay-3", "status": "HEALTHY", "model": "NVMe-Enterprise-3.84TB", "capacity_gb": 3840, "used_gb": 1400, "temperature_c": 32, "wear_pct": 8,  "chunks_count": 39, "chunk_ids": []},
        ],
    },
]

MOCK_OBJECTS = [
    {
        "key": "datasets/production-weights-v2.bin",
        "size": 4194304,
        "chunks": [
            {"chunk_id": "chk-weights-00", "index": 0, "size": 1048576, "sha256": "4a7d1ed414474e4033ac29ccb8653d9b048a82d396a0f4ed056c97d7", "replicas": ["node-1", "node-2", "node-3"], "is_parity": False},
            {"chunk_id": "chk-weights-01", "index": 1, "size": 1048576, "sha256": "b5d7d9a19c676d1e431804c4547bebb07b8b7095c9a6ff336f328f41", "replicas": ["node-2", "node-3", "node-4"], "is_parity": False},
            {"chunk_id": "chk-weights-02", "index": 2, "size": 1048576, "sha256": "8d3e91b5c4f2e71829bb57201c13d7890a56e6d1838cf451a92e1215", "replicas": ["node-3", "node-4", "node-1"], "is_parity": False},
            {"chunk_id": "chk-weights-03", "index": 3, "size": 1048576, "sha256": "e2f0a1c97b8319dc6e82a3b04c8e71510fa9504e927c32bf28a9b6c0", "replicas": ["node-4", "node-1", "node-2"], "is_parity": True},
        ],
        "created_at": "2026-09-25T14:32:00Z",
        "checksum": "4a7d1ed414474e4033ac29ccb8653d9b048a82d396a0f4ed056c97d7",
        "scheme": "reed-solomon (2+1)",
        "placement_policy": "ec",
    },
    {
        "key": "telemetry/audit-ledger-2026.parquet",
        "size": 2097152,
        "chunks": [
            {"chunk_id": "chk-ledger-00", "index": 0, "size": 1048576, "sha256": "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52dd", "replicas": ["node-1", "node-2", "node-4"], "is_parity": False},
            {"chunk_id": "chk-ledger-01", "index": 1, "size": 1048576, "sha256": "d4735e3a265e16eee03f59718b9b5d03019c07d8b6c51f90da3a666e", "replicas": ["node-2", "node-3", "node-4"], "is_parity": False},
        ],
        "created_at": "2026-09-25T18:15:22Z",
        "checksum": "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52dd",
        "scheme": "3x-replication",
        "placement_policy": "full",
    },
]

def _proxy_or_mock(path: str, fallback_data: Any) -> Any:
    """Attempts to fetch from VAULT_BACKEND_URL; falls back to mock data if offline."""
    if VAULT_BACKEND_URL and not VAULT_BACKEND_URL.startswith("http://localhost"):
        try:
            with httpx.Client(base_url=VAULT_BACKEND_URL, timeout=3.0) as client:
                res = client.get(path)
                if res.status_code == 200:
                    return res.json()
        except Exception:
            pass
    return fallback_data

@app.get("/api/status")
def get_status():
    fallback = {
        "active_objects": len(MOCK_OBJECTS),
        "coordinator": "vault-cloud-controller",
        "erasure_coding": True,
        "healthy_nodes": len([n for n in MOCK_NODES if n["status"] == "HEALTHY"]),
        "total_nodes": len(MOCK_NODES),
        "status": "HEALTHY",
        "replication_factor": 3,
        "write_quorum": 2,
        "read_quorum": 1,
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "total_capacity_bytes": 400000000000,
        "used_capacity_bytes": sum(n["used_space"] for n in MOCK_NODES),
    }
    return JSONResponse(content=_proxy_or_mock("/api/status", fallback))

@app.get("/api/nodes")
def get_nodes():
    return JSONResponse(content=_proxy_or_mock("/api/nodes", MOCK_NODES))

@app.get("/api/objects")
def get_objects():
    return JSONResponse(content=_proxy_or_mock("/api/objects", MOCK_OBJECTS))

@app.get("/api/topology")
def get_topology():
    ring_points = []
    step = int(4294967296 / (len(MOCK_NODES) * 8))
    token = 100000
    for n in MOCK_NODES:
        for _ in range(8):
            ring_points.append({"token": token, "node": n["id"]})
            token += step
    fallback = {
        "ring_type": "consistent_hash_vnode",
        "vnodes_per_node": 8,
        "physical_nodes": [n["id"] for n in MOCK_NODES],
        "ring_points": ring_points,
        "total_points": len(ring_points),
    }
    return JSONResponse(content=_proxy_or_mock("/api/topology", fallback))

@app.get("/api/metrics")
def get_metrics():
    now = time.strftime("%H:%M:%S")
    points = [
        {"timestamp": now, "write_mbps": 48.5, "read_mbps": 112.3, "iops": 4120, "avg_latency_ms": 1.25}
    ]
    return JSONResponse(content=_proxy_or_mock("/api/metrics", points))

@app.get("/api/events")
async def get_events():
    async def event_stream():
        # Stream keepalive and node heartbeats
        for i in range(15):
            now = time.strftime("%Y-%m-%dT%H:%M:%SZ")
            node_idx = (i % len(MOCK_NODES)) + 1
            heartbeat = {
                "type": "NODE_HEARTBEAT",
                "timestamp": now,
                "node_id": f"node-{node_idx}",
                "payload": {"status": "HEALTHY", "rtt_ms": round(1.1 + (i * 0.04), 2)},
            }
            yield f"data: {json.dumps(heartbeat)}\n\n"
            await asyncio.sleep(2)

    import json
    return StreamingResponse(event_stream(), media_type="text/event-stream")

@app.post("/api/admin/nodes/{node_id}/kill")
def kill_node(node_id: str):
    for n in MOCK_NODES:
        if n["id"] == node_id:
            n["status"] = "DEAD"
            return {"success": True, "message": f"Node {node_id} simulated failure"}
    return {"success": True, "message": f"Node {node_id} marked offline"}

@app.post("/api/admin/chunks/{chunk_id}/corrupt")
def corrupt_chunk(chunk_id: str):
    return {"success": True, "message": f"Chunk {chunk_id} bit-rot injected"}

@app.delete("/api/objects/{object_key}")
def delete_object(object_key: str):
    global MOCK_OBJECTS
    MOCK_OBJECTS = [o for o in MOCK_OBJECTS if o["key"] != object_key]
    return {"success": True, "deleted": object_key}

@app.post("/api/upload")
async def upload_object(file: UploadFile = File(...), placement: str = Form("full")):
    import hashlib
    content = await file.read()
    digest = hashlib.sha256(content).hexdigest()
    new_obj = {
        "key": file.filename or "uploaded_object.bin",
        "size": len(content),
        "chunks": [
            {
                "chunk_id": f"chk-{digest[:8]}-00",
                "index": 0,
                "size": len(content),
                "sha256": digest,
                "replicas": ["node-1", "node-2", "node-3"],
                "is_parity": False,
            }
        ],
        "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "checksum": digest,
        "scheme": "reed-solomon (2+1)" if placement == "ec" else "3x-replication",
        "placement_policy": placement,
    }
    MOCK_OBJECTS.append(new_obj)
    return {"success": True, "object": new_obj}

@app.get("/health")
def health_root():
    return {"status": "ok", "platform": "vercel-serverless"}
