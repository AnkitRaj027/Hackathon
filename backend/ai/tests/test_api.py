from fastapi.testclient import TestClient

from backend.ai.agent import ModelReply, VaultAIAgent
from backend.ai.tests.conftest import FakeModel, tool_reply
from backend.main import create_app


def test_chat_approval_and_rejection_api_flow() -> None:
    model = FakeModel([tool_reply("delete_object", {"object_id": "report.pdf"})])
    client = TestClient(create_app(VaultAIAgent(model=model)))
    pending = client.post("/api/ai/chat", json={"message": "Delete report.pdf"}).json()
    assert pending["type"] == "approval_required"
    response = client.post(
        f"/api/ai/approve/{pending['action_id']}",
        json={"approved": True, "tool": "remove_node", "arguments": {"node_id": "node1"}},
    )
    assert response.status_code == 200
    assert response.json()["tool"] == "delete_object"


def test_unapproved_chat_does_not_call_mutating_tool() -> None:
    model = FakeModel([tool_reply("repair_replica", {"object_id": "report.pdf"})])
    agent = VaultAIAgent(model=model)
    client = TestClient(create_app(agent))
    response = client.post("/api/ai/chat", json={"message": "Repair report.pdf"})
    assert response.json()["type"] == "approval_required"
    assert agent.service.objects["report.pdf"]["status"] == "healthy"


def test_reject_endpoint_cancels_pending_action() -> None:
    model = FakeModel([tool_reply("delete_object", {"object_id": "report.pdf"})])
    agent = VaultAIAgent(model=model)
    client = TestClient(create_app(agent))
    pending = client.post("/api/ai/chat", json={"message": "Delete report.pdf"}).json()
    rejected = client.post(f"/api/ai/reject/{pending['action_id']}")
    assert rejected.status_code == 200
    assert rejected.json()["success"] is False
    assert "report.pdf" in agent.service.objects


def test_api_node_failure_then_approved_repair_is_verified() -> None:
    model = FakeModel([
        tool_reply("simulate_node_failure", {"node_id": "node3"}),
        ModelReply(text="Node3 is unavailable."),
        tool_reply("get_object_metadata", {"object_id": "object-1"}),
        ModelReply(text="object-1 has two healthy replicas and is degraded."),
        tool_reply("repair_replica", {"object_id": "object-1"}),
        ModelReply(text="Repair complete."),
    ])
    agent = VaultAIAgent(model=model)
    client = TestClient(create_app(agent))

    failure = client.post("/api/ai/chat", json={"message": "Simulate failure of node3"}).json()
    assert failure["type"] == "approval_required"
    assert client.post(f"/api/ai/approve/{failure['action_id']}", json={"approved": True}).json()["success"]

    diagnosis = client.post("/api/ai/chat", json={"message": "Show metadata for object-1"}).json()
    assert diagnosis["type"] == "answer"
    assert agent.service.get_object_metadata("object-1")["status"] == "degraded"

    repair = client.post("/api/ai/chat", json={"message": "Repair object-1"}).json()
    assert repair["type"] == "approval_required"
    assert repair["tool"] == "repair_replica"
    assert agent.service.get_object_metadata("object-1")["healthy_replica_count"] == 2

    result = client.post(f"/api/ai/approve/{repair['action_id']}", json={"approved": True}).json()
    assert result["success"] is True
    assert result["result"]["verification"]["healthy_replica_count"] == 3
    assert result["result"]["verification"]["integrity_status"] == "healthy"


def test_health_and_actions_endpoints() -> None:
    model = FakeModel([ModelReply(text="Please name an object.")])
    client = TestClient(create_app(VaultAIAgent(model=model)))
    assert client.get("/api/ai/health").status_code == 200
    assert client.get("/api/ai/actions").json() == {"actions": []}