from datetime import datetime, timedelta, timezone
from concurrent.futures import ThreadPoolExecutor

from backend.ai.permissions import ActionStatus
from backend.ai.tests.conftest import SpyVaultService, tool_reply
from backend.ai.agent import ModelReply, VaultAIAgent


def test_read_only_command_executes_and_returns_model_answer(agent_factory) -> None:
    agent, model = agent_factory([
        tool_reply("get_node_status", {}),
        ModelReply(text="Vault currently has 4 healthy nodes.", interaction_id="interaction-2"),
    ])
    response = agent.chat("Show node health.")
    assert response["type"] == "answer"
    assert response["message"] == "Vault currently has 4 healthy nodes."
    assert model.submitted_results[0]["result"]["healthy_count"] == 4


def test_modify_command_waits_for_approval_then_executes(agent_factory) -> None:
    agent, model = agent_factory([
        tool_reply("repair_replica", {"object_id": "report.pdf"}),
        ModelReply(text="Replica repair completed successfully.", interaction_id="interaction-2"),
    ])
    pending = agent.chat("Repair report.pdf.")
    assert pending["type"] == "approval_required"
    assert pending["risk_level"] == "MEDIUM"
    assert agent.service.objects["report.pdf"]["status"] == "healthy"
    result = agent.approve(pending["action_id"])
    assert result["success"] is True
    assert result["message"] == "Replica repair completed successfully."
    assert model.submitted_results[0]["interaction_id"] == "interaction-1"
    assert agent.pending_actions[pending["action_id"]].status is ActionStatus.EXECUTED


def test_destructive_command_never_runs_before_explicit_approval(agent_factory) -> None:
    service = SpyVaultService()
    agent, _ = agent_factory([tool_reply("delete_object", {"object_id": "report.pdf"})], service=service)
    pending = agent.chat("Delete report.pdf.")
    assert pending["risk_level"] == "HIGH"
    assert "permanently" in pending["reason"]
    assert service.calls == []
    assert "report.pdf" in service.objects
    agent.approve(pending["action_id"])
    assert service.calls == [("delete_object", {"object_id": "report.pdf"})]


def test_rejected_action_is_not_executed(agent_factory) -> None:
    service = SpyVaultService()
    agent, _ = agent_factory([tool_reply("delete_object", {"object_id": "report.pdf"})], service=service)
    pending = agent.chat("Delete report.pdf.")
    response = agent.reject(pending["action_id"])
    assert response["success"] is False
    assert service.calls == []
    assert "report.pdf" in service.objects


def test_unknown_tool_is_not_routed(agent_factory) -> None:
    agent, _ = agent_factory([tool_reply("unknown_tool", {})])
    assert agent.chat("Do something") ["type"] == "error"


def test_invalid_object_and_invalid_node_return_errors(agent_factory) -> None:
    agent, _ = agent_factory([tool_reply("get_object_metadata", {"object_id": "absent.pdf"})])
    assert "was not found" in agent.chat("Show absent.pdf") ["message"]
    agent, _ = agent_factory([tool_reply("restore_node", {"node_id": "node99"})])
    assert "was not found" in agent.chat("Restore node99") ["message"]


def test_invalid_replication_factor_is_rejected(agent_factory) -> None:
    agent, _ = agent_factory([tool_reply("update_replication_factor", {"object_id": "report.pdf", "replication_factor": 0})])
    assert "at least 1" in agent.chat("Set replication to zero") ["message"]


def test_gemini_failure_is_safe_and_logged(agent_factory) -> None:
    agent, _ = agent_factory(error=RuntimeError("private transport detail"))
    response = agent.chat("Show node health.")
    assert response["type"] == "error"
    assert "private transport detail" not in response["message"]


def test_tool_execution_failure_is_safe(agent_factory) -> None:
    class FailingService(SpyVaultService):
        def get_node_status(self):
            raise RuntimeError("database connection string")

    agent, _ = agent_factory([tool_reply("get_node_status", {})], service=FailingService())
    response = agent.chat("Show node health")
    assert response["type"] == "error"
    assert "database connection string" not in response["message"]


def test_expired_approval_cannot_execute(agent_factory) -> None:
    agent, _ = agent_factory([tool_reply("repair_replica", {"object_id": "report.pdf"})])
    pending = agent.chat("Repair report.pdf")
    action = agent.pending_actions[pending["action_id"]]
    agent.pending_actions[pending["action_id"]] = action.model_copy(
        update={"expires_at": datetime.now(timezone.utc) - timedelta(seconds=1)}
    )
    try:
        agent.approve(pending["action_id"])
        raise AssertionError("Expected expired approval to be rejected")
    except Exception as exc:
        assert getattr(exc, "status_code", None) == 410
    assert agent.pending_actions[pending["action_id"]].status is ActionStatus.EXPIRED


def test_duplicate_concurrent_approval_executes_once(agent_factory) -> None:
    service = SpyVaultService()
    agent, _ = agent_factory([tool_reply("delete_object", {"object_id": "report.pdf"})], service=service)
    pending = agent.chat("Delete report.pdf")

    def attempt_approval():
        try:
            return agent.approve(pending["action_id"])["success"]
        except Exception as exc:
            return getattr(exc, "status_code", None)

    with ThreadPoolExecutor(max_workers=2) as pool:
        outcomes = list(pool.map(lambda _: attempt_approval(), range(2)))
    assert set(outcomes) == {409, True}
    assert len(service.calls) == 1


def test_mock_corruption_is_detected_and_repair_restores_integrity() -> None:
    service = SpyVaultService()
    service.simulate_corruption("report.pdf", "node2")
    assert service.verify_integrity("report.pdf")["corrupted_replicas"] == ["node2"]
    service.repair_replica("report.pdf")
    assert service.verify_integrity("report.pdf")["integrity_status"] == "healthy"


def test_mock_rebalance_moves_replica_toward_lower_utilization() -> None:
    service = SpyVaultService()
    result = service.rebalance_storage()
    assert result["moved_replica_count"] == 1
    assert "node4" in service.objects["report.pdf"]["replica_locations"]
    assert "node2" not in service.objects["report.pdf"]["replica_locations"]


def test_ambiguous_command_can_ask_for_clarification(agent_factory) -> None:
    agent, _ = agent_factory([ModelReply(text="Which file would you like me to repair?")])
    response = agent.chat("Repair it")
    assert response["type"] == "answer"
    assert response["message"].startswith("Which file")


def test_live_state_claim_without_tool_is_not_returned(agent_factory) -> None:
    agent, _ = agent_factory([ModelReply(text="report.pdf has 99 replicas.")])
    response = agent.chat("How many replicas does report.pdf have?")
    assert response["type"] == "error"
    assert "couldn't verify" in response["message"]


def test_model_conversation_context_is_chained(agent_factory) -> None:
    agent, model = agent_factory([
        ModelReply(text="report.pdf has 3 replicas.", interaction_id="turn-1"),
        ModelReply(text="I understand you mean report.pdf.", interaction_id="turn-2"),
    ])
    agent.chat("How many replicas does report.pdf have?", "session-1")
    agent.chat("Repair it", "session-1")
    assert model.previous_ids == [None, "turn-1"]