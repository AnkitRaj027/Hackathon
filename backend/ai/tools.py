from dataclasses import dataclass
from typing import Any, Callable

from .permissions import RiskLevel, destructive_warning
from .services.vault_service import VaultService


class ToolArgumentError(ValueError):
	pass


@dataclass(frozen=True)
class ToolDefinition:
	name: str
	description: str
	parameters: dict[str, Any]
	implementation: Callable[..., dict[str, Any]]
	risk_level: RiskLevel
	approval_required: bool
	reason: str

	def gemini_declaration(self) -> dict[str, Any]:
		return {
			"type": "function",
			"name": self.name,
			"description": self.description,
			"parameters": self.parameters,
		}


class ToolRegistry:
	def __init__(self, service: VaultService) -> None:
		self.service = service
		self.available_tools = self._build_tools()

	def _build_tools(self) -> dict[str, ToolDefinition]:
		registry: dict[str, ToolDefinition] = {}
		service = self.service

		def add(
			name: str,
			description: str,
			properties: dict[str, Any],
			required: list[str],
			implementation: Callable[..., dict[str, Any]],
			risk: RiskLevel,
		) -> None:
			reason = {
				RiskLevel.READ: "This operation only reads Vault state.",
				RiskLevel.MODIFY: "This operation will modify Vault state.",
				RiskLevel.DESTRUCTIVE: destructive_warning(name),
			}[risk]
			registry[name] = ToolDefinition(
				name=name,
				description=description,
				parameters={
					"type": "object",
					"properties": properties,
					"required": required,
					"additionalProperties": False,
				},
				implementation=implementation,
				risk_level=risk,
				approval_required=risk is not RiskLevel.READ,
				reason=reason,
			)

		string = lambda description: {"type": "string", "description": description}
		integer = lambda description: {"type": "integer", "description": description}
		add("get_node_status", "Get health and storage utilization for all nodes.", {}, [], service.get_node_status, RiskLevel.READ)
		add("list_objects", "List objects currently stored in Vault.", {}, [], service.list_objects, RiskLevel.READ)
		add("get_object_metadata", "Get metadata and replica placement for one object.", {"object_id": string("Exact Vault object ID.")}, ["object_id"], service.get_object_metadata, RiskLevel.READ)
		add("verify_integrity", "Compare replica hashes for one object.", {"object_id": string("Exact Vault object ID.")}, ["object_id"], service.verify_integrity, RiskLevel.READ)
		add("upload_object", "Register an object using a backend-owned file reference and replication factor.", {"file_reference": string("Opaque backend file reference; never a local path."), "replication_factor": integer("Number of replicas, from 1 to the current node count.")}, ["file_reference", "replication_factor"], service.upload_object, RiskLevel.MODIFY)
		add("repair_replica", "Repair missing or unhealthy replicas of an object.", {"object_id": string("Exact Vault object ID.")}, ["object_id"], service.repair_replica, RiskLevel.MODIFY)
		add("rebalance_storage", "Rebalance object placement across storage nodes.", {}, [], service.rebalance_storage, RiskLevel.MODIFY)
		add("simulate_node_failure", "Mark a node unavailable for failure testing.", {"node_id": string("Exact Vault node ID.")}, ["node_id"], service.simulate_node_failure, RiskLevel.MODIFY)
		add("restore_node", "Restore an unavailable node.", {"node_id": string("Exact Vault node ID.")}, ["node_id"], service.restore_node, RiskLevel.MODIFY)
		add("update_replication_factor", "Change an object's replication factor.", {"object_id": string("Exact Vault object ID."), "replication_factor": integer("New replica count, from 1 to the current node count.")}, ["object_id", "replication_factor"], service.update_replication_factor, RiskLevel.MODIFY)
		add("delete_object", "Permanently delete an object and its replicas.", {"object_id": string("Exact Vault object ID.")}, ["object_id"], service.delete_object, RiskLevel.DESTRUCTIVE)
		add("remove_node", "Permanently remove a storage node from Vault.", {"node_id": string("Exact Vault node ID.")}, ["node_id"], service.remove_node, RiskLevel.DESTRUCTIVE)
		return registry

	def declarations(self) -> list[dict[str, Any]]:
		return [tool.gemini_declaration() for tool in self.available_tools.values()]

	def get(self, name: str) -> ToolDefinition:
		try:
			return self.available_tools[name]
		except KeyError as exc:
			raise LookupError(f"Unknown Vault tool: {name}") from exc

	def validate_and_execute(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
		tool = self.get(name)
		validated = self.validate_arguments(tool, arguments)
		return tool.implementation(**validated)

	def validate_arguments(self, tool: ToolDefinition, arguments: dict[str, Any]) -> dict[str, Any]:
		if not isinstance(arguments, dict):
			raise ToolArgumentError("Tool arguments must be an object.")
		properties = tool.parameters["properties"]
		unexpected = set(arguments) - set(properties)
		missing = set(tool.parameters["required"]) - set(arguments)
		if unexpected:
			raise ToolArgumentError(f"Unexpected argument(s): {', '.join(sorted(unexpected))}.")
		if missing:
			raise ToolArgumentError(f"Missing required argument(s): {', '.join(sorted(missing))}.")

		clean = dict(arguments)
		for key, value in clean.items():
			expected = properties[key]["type"]
			if expected == "string":
				if not isinstance(value, str) or not value.strip():
					raise ToolArgumentError(f"{key} must be a non-empty string.")
				clean[key] = value.strip()
			elif expected == "integer":
				if isinstance(value, bool) or not isinstance(value, int):
					raise ToolArgumentError(f"{key} must be an integer.")
				if value < 1:
					raise ToolArgumentError(f"{key} must be at least 1.")

		if "file_reference" in clean and ("/" in clean["file_reference"] or "\\" in clean["file_reference"] or ".." in clean["file_reference"]):
			raise ToolArgumentError("file_reference must be an opaque backend reference, not a filesystem path.")

		if "object_id" in clean:
			try:
				self.service.get_object_metadata(clean["object_id"])
			except ValueError as exc:
				raise ToolArgumentError(str(exc)) from exc
		if "node_id" in clean:
			nodes = self.service.get_node_status().get("nodes", [])
			if clean["node_id"] not in {node.get("id") for node in nodes}:
				raise ToolArgumentError(f"Node {clean['node_id']} was not found.")
		if "replication_factor" in clean:
			node_count = len(self.service.get_node_status().get("nodes", []))
			if clean["replication_factor"] > node_count:
				raise ToolArgumentError(f"replication_factor cannot exceed the current node count ({node_count}).")
		return clean
