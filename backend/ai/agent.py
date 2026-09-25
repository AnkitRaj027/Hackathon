import json
import logging
import re
import secrets
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from threading import RLock
from typing import Any, Protocol
from uuid import uuid4

from google import genai

from .audit import AuditLog
from .config import APPROVAL_TTL_SECONDS, GEMINI_API_KEY, GEMINI_MODEL
from .models import PendingAction, ToolCall
from .permissions import ActionStatus, RiskLevel, requires_approval
from .prompts import SYSTEM_PROMPT
from .rag.retriever import KnowledgeRetriever
from .services.mock_vault_service import MockVaultService
from .services.vault_service import VaultService
from .tools import ToolArgumentError, ToolRegistry


logger = logging.getLogger(__name__)


@dataclass
class ModelReply:
    text: str = ""
    tool_calls: list[ToolCall] | None = None
    interaction_id: str | None = None
    call_id: str | None = None


class ModelProvider(Protocol):
    def complete(
        self,
        message: str,
        tools: list[dict[str, Any]],
        previous_interaction_id: str | None,
        knowledge_context: list[dict[str, str]],
    ) -> ModelReply: ...

    def submit_tool_result(
        self,
        tool_name: str,
        call_id: str | None,
        result: dict[str, Any],
        interaction_id: str | None,
        tools: list[dict[str, Any]],
    ) -> ModelReply: ...


class GeminiInteractionsModel:
    def __init__(self, api_key: str | None = None, model: str | None = None) -> None:
        self.api_key = api_key if api_key is not None else GEMINI_API_KEY
        self.model = model or GEMINI_MODEL
        self._client: genai.Client | None = None

    def _get_client(self) -> genai.Client:
        if not self.api_key:
            raise RuntimeError("Gemini is not configured. Set GEMINI_API_KEY in backend/.env.")
        if self._client is None:
            self._client = genai.Client(api_key=self.api_key)
        return self._client

    def complete(
        self,
        message: str,
        tools: list[dict[str, Any]],
        previous_interaction_id: str | None,
        knowledge_context: list[dict[str, str]],
    ) -> ModelReply:
        enriched_message = message
        if knowledge_context:
            sources = "\n\n".join(
                f"[{item['source']}]\n{item['content']}" for item in knowledge_context
            )
            enriched_message = (
                "Retrieved general Vault documentation follows. It is not live state; "
                f"use tools for operational facts.\n{sources}\n\nUser request: {message}"
            )
        interaction = self._get_client().interactions.create(
            model=self.model,
            input=enriched_message,
            previous_interaction_id=previous_interaction_id,
            system_instruction=SYSTEM_PROMPT,
            tools=tools,
        )
        return self._as_reply(interaction)

    def submit_tool_result(
        self,
        tool_name: str,
        call_id: str | None,
        result: dict[str, Any],
        interaction_id: str | None,
        tools: list[dict[str, Any]],
    ) -> ModelReply:
        if not interaction_id or not call_id:
            raise RuntimeError("Gemini did not return the identifiers required to submit a tool result.")
        interaction = self._get_client().interactions.create(
            model=self.model,
            previous_interaction_id=interaction_id,
            system_instruction=SYSTEM_PROMPT,
            tools=tools,
            input=[{
                "type": "function_result",
                "name": tool_name,
                "call_id": call_id,
                "result": [{"type": "text", "text": json.dumps(result, default=str)}],
            }],
        )
        return self._as_reply(interaction)

    @staticmethod
    def _as_reply(interaction: Any) -> ModelReply:
        calls: list[ToolCall] = []
        call_id: str | None = None
        for step in getattr(interaction, "steps", []) or []:
            if getattr(step, "type", None) != "function_call":
                continue
            calls.append(ToolCall(name=step.name, arguments=step.arguments or {}))
            call_id = getattr(step, "id", None)
        return ModelReply(
            text=getattr(interaction, "output_text", "") or "",
            tool_calls=calls,
            interaction_id=getattr(interaction, "id", None),
            call_id=call_id,
        )


class ActionError(Exception):
    def __init__(self, message: str, status_code: int) -> None:
        super().__init__(message)
        self.status_code = status_code


class VaultAIAgent:
    def __init__(
        self,
        service: VaultService | None = None,
        model: ModelProvider | None = None,
        audit_log: AuditLog | None = None,
        retriever: KnowledgeRetriever | None = None,
        approval_ttl_seconds: int = APPROVAL_TTL_SECONDS,
    ) -> None:
        self.service = service or MockVaultService()
        self.tools = ToolRegistry(self.service)
        self.model = model or GeminiInteractionsModel()
        self.audit_log = audit_log or AuditLog()
        self.retriever = retriever or KnowledgeRetriever()
        self.approval_ttl_seconds = approval_ttl_seconds
        self.pending_actions: dict[str, PendingAction] = {}
        self.session_interactions: dict[str, str] = {}
        self._lock = RLock()

    def chat(self, message: str, session_id: str | None = None) -> dict[str, Any]:
        session_id = session_id or uuid4().hex
        grounded_by_tool = False
        self.audit_log.record(session_id=session_id, user_request=message, status="received")
        try:
            reply = self.model.complete(
                message=message,
                tools=self.tools.declarations(),
                previous_interaction_id=self.session_interactions.get(session_id),
                knowledge_context=self.retriever.retrieve(message),
            )
            for _ in range(4):
                calls = reply.tool_calls or []
                if not calls:
                    self._remember(session_id, reply.interaction_id)
                    answer = reply.text.strip() or "I couldn't produce a response. Please try again."
                    if self._requires_live_tool(message) and not grounded_by_tool and not self._is_clarification(answer):
                        return self._error(
                            "I couldn't verify the current Vault state or prepare that operation. Please clarify the target or try again.",
                            session_id,
                        )
                    self.audit_log.record(session_id=session_id, user_request=message, status="answered")
                    return {"type": "answer", "message": answer, "session_id": session_id}
                if len(calls) != 1:
                    return self._error("I received an unsupported multi-tool request. Please try one operation at a time.", session_id)

                call = calls[0]
                tool = self.tools.get(call.name)
                arguments = self.tools.validate_arguments(tool, call.arguments)
                if requires_approval(tool.risk_level):
                    pending = self._create_pending_action(
                        tool.name,
                        arguments,
                        tool.risk_level,
                        tool.reason,
                        session_id,
                        reply.interaction_id,
                        reply.call_id,
                    )
                    self._remember(session_id, reply.interaction_id)
                    self.audit_log.record(
                        session_id=session_id,
                        user_request=message,
                        tool=tool.name,
                        arguments=arguments,
                        risk_level=tool.risk_level.value,
                        approval_status="PENDING",
                        execution_status="NOT_EXECUTED",
                    )
                    return {
                        "type": "approval_required",
                        "action_id": pending.action_id,
                        "tool": pending.tool_name,
                        "arguments": pending.arguments,
                        "risk_level": pending.risk_level.value,
                        "reason": pending.reason,
                        "session_id": session_id,
                    }

                result = self.tools.validate_and_execute(tool.name, arguments)
                grounded_by_tool = True
                self.audit_log.record(
                    session_id=session_id,
                    user_request=message,
                    tool=tool.name,
                    arguments=arguments,
                    risk_level=tool.risk_level.value,
                    approval_status="NOT_REQUIRED",
                    execution_status="SUCCESS",
                    result_summary=result,
                )
                reply = self.model.submit_tool_result(
                    tool_name=tool.name,
                    call_id=reply.call_id,
                    result=result,
                    interaction_id=reply.interaction_id,
                    tools=self.tools.declarations(),
                )
            return self._error("The request needed too many consecutive tool calls. Please make it more specific.", session_id)
        except LookupError as exc:
            self.audit_log.record(session_id=session_id, user_request=message, status="unknown_tool", error=str(exc))
            return self._error("I couldn't match that request to an available Vault operation.", session_id)
        except ToolArgumentError as exc:
            self.audit_log.record(session_id=session_id, user_request=message, status="invalid_arguments", error=str(exc))
            return self._error(str(exc), session_id)
        except Exception:
            logger.exception("Vault AI chat failed for session %s", session_id)
            self.audit_log.record(session_id=session_id, user_request=message, status="failed", error="Model or tool processing failed")
            return self._error("Vault AI could not complete the request. Check the backend configuration and try again.", session_id)

    def approve(self, action_id: str) -> dict[str, Any]:
        approved = self._claim_pending(action_id, ActionStatus.APPROVED)
        try:
            tool = self.tools.get(approved.tool_name)
            if not requires_approval(tool.risk_level):
                raise ValueError("This operation is no longer eligible for approval.")
            arguments = self.tools.validate_arguments(tool, approved.arguments)
        except Exception as exc:
            self._update_action(approved.model_copy(update={"status": ActionStatus.FAILED}))
            self.audit_log.record(action_id=action_id, tool=approved.tool_name, approval_status="APPROVED", execution_status="FAILED", error=str(exc))
            raise ActionError("The stored action is no longer valid and was not executed.", 409) from exc

        self.audit_log.record(action_id=action_id, tool=tool.name, arguments=arguments, risk_level=tool.risk_level.value, approval_status="APPROVED")
        try:
            result = self.tools.validate_and_execute(tool.name, arguments)
            self._update_action(approved.model_copy(update={"status": ActionStatus.EXECUTED}))
            self.audit_log.record(action_id=action_id, tool=tool.name, approval_status="APPROVED", execution_status="SUCCESS", result_summary=result)
            message = self._success_message(tool.name, result)
            try:
                reply = self.model.submit_tool_result(
                    tool_name=tool.name,
                    call_id=approved.call_id,
                    result=result,
                    interaction_id=approved.interaction_id,
                    tools=self.tools.declarations(),
                )
                self._remember(approved.session_id or "", reply.interaction_id)
                message = reply.text.strip() or message
            except Exception:
                logger.exception("Vault operation succeeded but Gemini could not format the result")
            return {
                "type": "tool_result",
                "success": True,
                "tool": tool.name,
                "result": result,
                "message": message,
            }
        except Exception as exc:
            self._update_action(approved.model_copy(update={"status": ActionStatus.FAILED}))
            logger.exception("Approved Vault action %s failed", action_id)
            self.audit_log.record(action_id=action_id, tool=tool.name, approval_status="APPROVED", execution_status="FAILED", error=str(exc))
            return {"type": "tool_result", "success": False, "tool": tool.name, "message": "Approval was recorded, but Vault could not complete the operation."}

    def reject(self, action_id: str) -> dict[str, Any]:
        action = self._claim_pending(action_id, ActionStatus.REJECTED)
        self.audit_log.record(action_id=action_id, tool=action.tool_name, risk_level=action.risk_level.value, approval_status="REJECTED", execution_status="NOT_EXECUTED")
        return {"type": "tool_result", "success": False, "tool": action.tool_name, "message": "The Vault action was cancelled and was not executed."}

    def list_actions(self) -> list[dict[str, Any]]:
        with self._lock:
            return [action.model_dump(mode="json") for action in self.pending_actions.values()]

    def _create_pending_action(
        self,
        tool_name: str,
        arguments: dict[str, Any],
        risk_level: RiskLevel,
        reason: str,
        session_id: str,
        interaction_id: str | None,
        call_id: str | None,
    ) -> PendingAction:
        now = datetime.now(timezone.utc)
        action = PendingAction(
            action_id=secrets.token_urlsafe(24),
            tool_name=tool_name,
            arguments=dict(arguments),
            risk_level=risk_level,
            reason=reason,
            created_at=now,
            expires_at=now + timedelta(seconds=self.approval_ttl_seconds),
            session_id=session_id,
            interaction_id=interaction_id,
            call_id=call_id,
        )
        with self._lock:
            self.pending_actions[action.action_id] = action
        return action

    def _get_pending(self, action_id: str) -> PendingAction:
        with self._lock:
            action = self.pending_actions.get(action_id)
        if action is None:
            raise ActionError("Pending action was not found.", 404)
        return action

    def _update_action(self, action: PendingAction) -> None:
        with self._lock:
            self.pending_actions[action.action_id] = action

    def _claim_pending(self, action_id: str, new_status: ActionStatus) -> PendingAction:
        with self._lock:
            action = self.pending_actions.get(action_id)
            if action is None:
                raise ActionError("Pending action was not found.", 404)
            if action.status is not ActionStatus.PENDING:
                raise ActionError(f"This action is already {action.status.value.lower()}.", 409)
            if datetime.now(timezone.utc) >= action.expires_at:
                self.pending_actions[action_id] = action.model_copy(update={"status": ActionStatus.EXPIRED})
                raise ActionError("This approval has expired. Submit the request again.", 410)
            claimed = action.model_copy(update={"status": new_status})
            self.pending_actions[action_id] = claimed
            return claimed

    def _remember(self, session_id: str, interaction_id: str | None) -> None:
        if interaction_id:
            self.session_interactions[session_id] = interaction_id

    @staticmethod
    def _success_message(tool_name: str, result: dict[str, Any]) -> str:
        if tool_name == "repair_replica":
            return f"Replica repair completed for {result.get('object_id', 'the object')}."
        if tool_name == "delete_object":
            return f"Object {result.get('object_id', '')} was deleted."
        if tool_name == "rebalance_storage":
            return "Storage rebalancing completed."
        return f"{tool_name.replace('_', ' ').capitalize()} completed successfully."

    @staticmethod
    def _error(message: str, session_id: str) -> dict[str, Any]:
        return {"type": "error", "message": message, "session_id": session_id}

    @staticmethod
    def _requires_live_tool(message: str) -> bool:
        normalized = message.lower()
        state_terms = (
            "status", "health", "healthy", "unavailable", "currently down", "node ",
            "replica", "replicas", "metadata", "integrity", "degraded", "corrupt",
            "storage utilization", "storage usage", "how many nodes", "how many replicas",
        )
        action_terms = (
            "repair", "rebalance", "simulate failure", "restore node", "replication factor",
            "delete ", "remove node", "upload ",
        )
        return any(term in normalized for term in (*state_terms, *action_terms))

    @staticmethod
    def _is_clarification(message: str) -> bool:
        normalized = message.strip().lower()
        return normalized.endswith("?") or normalized.startswith((
            "which ", "what ", "could you clarify", "please specify", "please clarify",
        ))