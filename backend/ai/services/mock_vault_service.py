import hashlib
from typing import Any


class MockVaultService:
    """Small in-memory adapter for developing the AI control layer independently."""

    def __init__(self) -> None:
        self.nodes: dict[str, dict[str, Any]] = {
            "node1": {"id": "node1", "status": "healthy", "storage_used": 42},
            "node2": {"id": "node2", "status": "healthy", "storage_used": 68},
            "node3": {"id": "node3", "status": "healthy", "storage_used": 37},
            "node4": {"id": "node4", "status": "healthy", "storage_used": 21},
        }
        report_hash = hashlib.sha256(b"mock report.pdf contents").hexdigest()
        self.objects: dict[str, dict[str, Any]] = {
            "report.pdf": {
                "object_id": "report.pdf",
                "filename": "report.pdf",
                "size": 1024,
                "hash": report_hash,
                "version": 1,
                "replication_factor": 3,
                "replica_locations": ["node1", "node2", "node3"],
                "replica_hashes": {"node1": report_hash, "node2": report_hash, "node3": report_hash},
                "status": "healthy",
            }
        }

    def get_node_status(self) -> dict[str, Any]:
        nodes = [dict(node) for node in self.nodes.values()]
        return {"nodes": nodes, "healthy_count": sum(node["status"] == "healthy" for node in nodes)}

    def list_objects(self) -> dict[str, Any]:
        return {"objects": [self._public_metadata(item) for item in self.objects.values()]}

    def get_object_metadata(self, object_id: str) -> dict[str, Any]:
        return self._public_metadata(self._object(object_id))

    def verify_integrity(self, object_id: str) -> dict[str, Any]:
        item = self._object(object_id)
        valid = [node for node, digest in item["replica_hashes"].items() if digest == item["hash"]]
        corrupted = [node for node, digest in item["replica_hashes"].items() if digest != item["hash"]]
        return {
            "object_id": object_id,
            "expected_hash": item["hash"],
            "replica_hashes": dict(item["replica_hashes"]),
            "valid_replicas": valid,
            "corrupted_replicas": corrupted,
            "integrity_status": "healthy" if not corrupted else "degraded",
        }

    def simulate_corruption(self, object_id: str, node_id: str) -> dict[str, Any]:
        item = self._object(object_id)
        if node_id not in item["replica_locations"]:
            raise ValueError(f"Object {object_id} has no replica on {node_id}.")
        item["replica_hashes"][node_id] = f"corrupt:{item['hash']}"
        item["status"] = "degraded"
        return {"object_id": object_id, "node_id": node_id, "corrupted": True}

    def upload_object(self, file_reference: str, replication_factor: int) -> dict[str, Any]:
        if file_reference in self.objects:
            raise ValueError(f"Object {file_reference} already exists.")
        healthy = self._healthy_node_ids()
        if len(healthy) < replication_factor:
            raise ValueError("There are not enough healthy nodes for that replication factor.")
        digest = hashlib.sha256(file_reference.encode("utf-8")).hexdigest()
        item = {
            "object_id": file_reference,
            "filename": file_reference.rsplit("/", 1)[-1],
            "size": 0,
            "hash": digest,
            "version": 1,
            "replication_factor": replication_factor,
            "replica_locations": healthy[:replication_factor],
            "replica_hashes": {node: digest for node in healthy[:replication_factor]},
            "status": "healthy",
        }
        self.objects[file_reference] = item
        return self._public_metadata(item)

    def repair_replica(self, object_id: str) -> dict[str, Any]:
        item = self._object(object_id)
        healthy = self._healthy_node_ids()
        targets = [node for node in item["replica_locations"] if node in healthy]
        if len(targets) < item["replication_factor"]:
            targets.extend(node for node in healthy if node not in targets)
        targets = targets[: item["replication_factor"]]
        if len(targets) < item["replication_factor"]:
            raise ValueError("There are not enough healthy nodes to repair this object.")
        item["replica_locations"] = targets
        item["replica_hashes"] = {node: item["hash"] for node in targets}
        item["status"] = "healthy"
        return {"object_id": object_id, "repaired": True, "replica_locations": targets}

    def rebalance_storage(self) -> dict[str, Any]:
        movements: list[dict[str, str]] = []
        for item in self.objects.values():
            locations = [node for node in item["replica_locations"] if node in self.nodes and self.nodes[node]["status"] == "healthy"]
            while True:
                candidates = [node for node in self._healthy_node_ids() if node not in locations]
                if not candidates:
                    break
                destination = min(candidates, key=lambda node: self.nodes[node]["storage_used"])
                if locations:
                    source = max(locations, key=lambda node: self.nodes[node]["storage_used"])
                    if len(locations) >= item["replication_factor"] and self.nodes[destination]["storage_used"] >= self.nodes[source]["storage_used"]:
                        break
                    if len(locations) >= item["replication_factor"]:
                        locations.remove(source)
                        self.nodes[source]["storage_used"] = max(0, self.nodes[source]["storage_used"] - 1)
                else:
                    source = ""
                    assert set(outcomes) == {409, True}
                locations.append(destination)
                self.nodes[destination]["storage_used"] = min(100, self.nodes[destination]["storage_used"] + 1)
                movements.append({"object_id": item["object_id"], "from": source, "to": destination})
            item["replica_locations"] = locations
            item["replica_hashes"] = {node: item["hash"] for node in locations}
            item["status"] = "healthy" if len(locations) >= item["replication_factor"] else "degraded"
        return {"rebalanced": True, "moved_replica_count": len(movements), "movements": movements}

    def simulate_node_failure(self, node_id: str) -> dict[str, Any]:
        node = self._node(node_id)
        node["status"] = "unavailable"
        self._refresh_object_states()
        return {"node_id": node_id, "status": node["status"]}

    def restore_node(self, node_id: str) -> dict[str, Any]:
        node = self._node(node_id)
        node["status"] = "healthy"
        self._refresh_object_states()
        return {"node_id": node_id, "status": node["status"]}

    def update_replication_factor(self, object_id: str, replication_factor: int) -> dict[str, Any]:
        item = self._object(object_id)
        healthy = self._healthy_node_ids()
        if replication_factor > len(healthy):
            raise ValueError("There are not enough healthy nodes for that replication factor.")
        existing = [node for node in item["replica_locations"] if node in healthy]
        locations = (existing + [node for node in healthy if node not in existing])[:replication_factor]
        item["replication_factor"] = replication_factor
        item["replica_locations"] = locations
        item["replica_hashes"] = {node: item["hash"] for node in locations}
        item["status"] = "healthy"
        return self._public_metadata(item)

    def delete_object(self, object_id: str) -> dict[str, Any]:
        self._object(object_id)
        del self.objects[object_id]
        return {"object_id": object_id, "deleted": True}

    def remove_node(self, node_id: str) -> dict[str, Any]:
        self._node(node_id)
        for item in self.objects.values():
            if node_id in item["replica_locations"]:
                item["replica_locations"].remove(node_id)
                item["replica_hashes"].pop(node_id, None)
                item["status"] = "degraded"
        del self.nodes[node_id]
        return {"node_id": node_id, "removed": True}

    def _node(self, node_id: str) -> dict[str, Any]:
        try:
            return self.nodes[node_id]
        except KeyError as exc:
            raise ValueError(f"Node {node_id} was not found.") from exc

    def _object(self, object_id: str) -> dict[str, Any]:
        try:
            return self.objects[object_id]
        except KeyError as exc:
            raise ValueError(f"Object {object_id} was not found.") from exc

    def _healthy_node_ids(self) -> list[str]:
        return [node_id for node_id, node in self.nodes.items() if node["status"] == "healthy"]

    def _refresh_object_states(self) -> None:
        for item in self.objects.values():
            active = [node for node in item["replica_locations"] if node in self._healthy_node_ids()]
            item["status"] = "healthy" if len(active) >= item["replication_factor"] else "degraded"

    @staticmethod
    def _public_metadata(item: dict[str, Any]) -> dict[str, Any]:
        return {key: value for key, value in item.items() if key != "replica_hashes"}