from collections.abc import Iterable
from typing import Any

import pytest

from backend.ai.agent import ModelReply, VaultAIAgent
from backend.ai.models import ToolCall
from backend.ai.services.mock_vault_service import MockVaultService


class FakeModel:
    def __init__(self, replies: Iterable[ModelReply] = (), error: Exception | None = None) -> None:
        self.replies = list(replies)
        self.error = error
        self.previous_ids: list[str | None] = []
        self.submitted_results: list[dict[str, Any]] = []
        self.submitted_batches: list[list[dict[str, Any]]] = []
        self.api_key = None
        self.model = "fake"

    def complete(self, message, tools, previous_interaction_id, knowledge_context):
        self.previous_ids.append(previous_interaction_id)
        if self.error:
            raise self.error
        return self.replies.pop(0)

    def submit_tool_result(self, tool_name, call_id, result, interaction_id, tools):
        self.submitted_results.append({"tool": tool_name, "call_id": call_id, "interaction_id": interaction_id, "result": result})
        return self.replies.pop(0) if self.replies else ModelReply(text="Operation result received.")

    def submit_tool_results(self, results, interaction_id, tools):
        self.submitted_batches.append(results)
        self.submitted_results.extend(
            {**item, "interaction_id": interaction_id} for item in results
        )
        return self.replies.pop(0) if self.replies else ModelReply(text="Operation results received.")


class SpyVaultService(MockVaultService):
    def __init__(self) -> None:
        super().__init__()
        self.calls: list[tuple[str, dict[str, Any]]] = []

    def delete_object(self, object_id: str) -> dict[str, Any]:
        self.calls.append(("delete_object", {"object_id": object_id}))
        return super().delete_object(object_id)


def tool_reply(name: str, arguments: dict[str, Any], interaction_id: str = "interaction-1") -> ModelReply:
    return ModelReply(
        tool_calls=[ToolCall(name=name, arguments=arguments)],
        interaction_id=interaction_id,
        call_id="call-1",
    )


@pytest.fixture
def agent_factory():
    def create(replies=(), service=None, error=None, ttl=300, max_tool_iterations=6):
        model = FakeModel(replies, error)
        agent = VaultAIAgent(
            service=service or MockVaultService(),
            model=model,
            approval_ttl_seconds=ttl,
            max_tool_iterations=max_tool_iterations,
        )
        return agent, model

    return create