import pytest

from backend.ai.services.mock_vault_service import MockVaultService
from backend.ai.tools import ToolRegistry


def test_registry_exposes_all_required_tools() -> None:
    registry = ToolRegistry(MockVaultService())
    assert len(registry.available_tools) == 12
    assert {"get_node_status", "verify_integrity", "remove_node"} <= set(registry.available_tools)


def test_read_tool_returns_mock_node_state() -> None:
    result = ToolRegistry(MockVaultService()).validate_and_execute("get_node_status", {})
    assert result["healthy_count"] == 4


@pytest.mark.parametrize(
    ("name", "arguments", "message"),
    [
        ("get_object_metadata", {"object_id": "missing.pdf"}, "Object missing.pdf was not found"),
        ("restore_node", {"node_id": "node99"}, "Node node99 was not found"),
        ("update_replication_factor", {"object_id": "report.pdf", "replication_factor": 5}, "cannot exceed"),
        ("update_replication_factor", {"object_id": "report.pdf", "replication_factor": True}, "must be an integer"),
    ],
)
def test_tool_validation_rejects_bad_targets_and_values(name, arguments, message) -> None:
    with pytest.raises(ValueError, match=message):
        ToolRegistry(MockVaultService()).validate_and_execute(name, arguments)


def test_unknown_tool_is_rejected() -> None:
    with pytest.raises(LookupError, match="Unknown Vault tool"):
        ToolRegistry(MockVaultService()).validate_and_execute("run_python", {})


def test_upload_reference_cannot_be_a_filesystem_path() -> None:
    with pytest.raises(ValueError, match="not a filesystem path"):
        ToolRegistry(MockVaultService()).validate_and_execute(
            "upload_object", {"file_reference": "C:\\private\\report.pdf", "replication_factor": 2}
        )