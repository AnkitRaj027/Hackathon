import logging
from typing import Any
import httpx

from .mock_vault_service import MockVaultService

logger = logging.getLogger(__name__)


class HttpVaultService(MockVaultService):
    """Connects the AI Agent to the real Vault Go coordinator REST API.

    Inherits from MockVaultService so state attributes (nodes, objects)
    and fallback operations remain fully available if the cluster is offline.
    """

    def __init__(self, base_url: str = "http://localhost:8085", timeout: float = 5.0) -> None:
        super().__init__()
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

    def _client(self) -> httpx.Client:
        return httpx.Client(base_url=self.base_url, timeout=self.timeout)

    def is_live(self) -> bool:
        try:
            with self._client() as client:
                res = client.get("/health")
                return res.status_code == 200
        except Exception:
            return False

    def get_node_status(self) -> dict[str, Any]:
        try:
            with self._client() as client:
                res = client.get("/api/nodes")
                if res.status_code == 200:
                    raw_nodes = res.json() or []
                    nodes = []
                    healthy_count = 0
                    for n in raw_nodes:
                        status_str = str(n.get("status", "")).upper()
                        is_healthy = status_str in ("HEALTHY", "ALIVE")
                        if is_healthy:
                            healthy_count += 1
                        nodes.append({
                            "id": n.get("id"),
                            "status": "healthy" if is_healthy else "unhealthy",
                            "address": n.get("address"),
                            "storage_used": int(n.get("used_space", 42)),
                            "total_space": int(n.get("total_space", 100)),
                        })
                    return {"nodes": nodes, "healthy_count": healthy_count}
        except Exception as e:
            logger.debug("Live Vault not reachable at /api/nodes: %s. Using local state.", e)
        return super().get_node_status()

    def list_objects(self) -> dict[str, Any]:
        try:
            with self._client() as client:
                res = client.get("/api/objects")
                if res.status_code == 200:
                    items = res.json() or []
                    formatted = []
                    for item in items:
                        replicas = item.get("replicas") or []
                        locations = [r.get("node_id") for r in replicas if r.get("node_id")]
                        formatted.append({
                            "object_id": item.get("key") or item.get("id"),
                            "filename": item.get("key"),
                            "size": item.get("size", 0),
                            "hash": item.get("checksum", ""),
                            "version": 1,
                            "replication_factor": max(len(locations), 1),
                            "replica_locations": locations,
                            "status": "healthy",
                        })
                    return {"objects": formatted}
        except Exception as e:
            logger.debug("Live Vault not reachable at /api/objects: %s. Using local state.", e)
        return super().list_objects()

    def get_object_metadata(self, object_id: str) -> dict[str, Any]:
        try:
            with self._client() as client:
                res = client.get(f"/api/objects/{object_id}")
                if res.status_code == 200:
                    item = res.json()
                    replicas = item.get("replicas") or []
                    locations = [r.get("node_id") for r in replicas if r.get("node_id")]
                    rep_hashes = {r.get("node_id"): item.get("checksum", "") for r in replicas if r.get("node_id")}
                    return {
                        "object_id": item.get("key") or item.get("id"),
                        "filename": item.get("key"),
                        "size": item.get("size", 0),
                        "hash": item.get("checksum", ""),
                        "version": 1,
                        "replication_factor": max(len(locations), 1),
                        "replica_locations": locations,
                        "replica_hashes": rep_hashes,
                        "status": "healthy",
                    }
        except Exception as e:
            logger.debug("Live Vault object %s query: %s. Using local state.", object_id, e)
        return super().get_object_metadata(object_id)

    def verify_integrity(self, object_id: str) -> dict[str, Any]:
        if not self.is_live():
            return super().verify_integrity(object_id)
        try:
            meta = self.get_object_metadata(object_id)
            node_status = self.get_node_status()
            healthy_node_ids = {n["id"] for n in node_status["nodes"] if n["status"] == "healthy"}
            locations = meta.get("replica_locations", [])
            valid = [n for n in locations if n in healthy_node_ids]
            unavailable = [n for n in locations if n not in healthy_node_ids]
            req_rf = meta.get("replication_factor", len(locations))
            missing = max(0, req_rf - len(valid))
            return {
                "object_id": object_id,
                "expected_hash": meta.get("hash", ""),
                "replica_hashes": meta.get("replica_hashes", {}),
                "valid_replicas": valid,
                "corrupted_replicas": [],
                "unavailable_replicas": unavailable,
                "healthy_replica_count": len(valid),
                "required_replica_count": req_rf,
                "missing_replica_count": missing,
                "integrity_status": "healthy" if missing == 0 else "degraded",
            }
        except Exception:
            return super().verify_integrity(object_id)

    def simulate_node_failure(self, node_id: str) -> dict[str, Any]:
        super_res = super().simulate_node_failure(node_id)
        try:
            with self._client() as client:
                res = client.post(f"/api/admin/nodes/{node_id}/kill")
                if res.status_code == 200:
                    super_res["live_cluster_notified"] = True
        except Exception as e:
            logger.debug("Live cluster node kill failed: %s", e)
        return super_res

    def restore_node(self, node_id: str) -> dict[str, Any]:
        super_res = super().restore_node(node_id)
        try:
            with self._client() as client:
                res = client.post(f"/api/admin/nodes/{node_id}/start")
                if res.status_code == 200:
                    super_res["live_cluster_notified"] = True
        except Exception as e:
            logger.debug("Live cluster node start failed: %s", e)
        return super_res

    def delete_object(self, object_id: str) -> dict[str, Any]:
        super_res = super().delete_object(object_id)
        try:
            with self._client() as client:
                res = client.delete(f"/api/objects/{object_id}")
                if res.status_code in (200, 204):
                    super_res["live_cluster_notified"] = True
        except Exception as e:
            logger.debug("Live cluster delete failed: %s", e)
        return super_res
