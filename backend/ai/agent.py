import json
import logging
import os
import re
import secrets
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from threading import RLock
from typing import Any, Protocol
from uuid import uuid4

import httpx
from google import genai

from .audit import AuditLog
from .config import (
    APPROVAL_TTL_SECONDS,
    GEMINI_API_KEY,
    GEMINI_MODEL,
    LLM_PROVIDER,
    MISTRAL_API_KEY,
    MISTRAL_BASE_URL,
    MISTRAL_MODEL,
    VAULT_BACKEND_URL,
)
from .models import PendingAction, ToolCall
from .permissions import ActionStatus, RiskLevel, requires_approval
from .prompts import SYSTEM_PROMPT
from .rag.retriever import KnowledgeRetriever
from .services.http_vault_service import HttpVaultService
from .services.mock_vault_service import MockVaultService
from .services.vault_service import VaultService
from .tools import ToolArgumentError, ToolRegistry


logger = logging.getLogger(__name__)
DEFAULT_MAX_TOOL_ITERATIONS = 6


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

    def submit_tool_results(
        self,
        results: list[dict[str, Any]],
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
        if not call_id:
            raise RuntimeError("Gemini did not return the identifier required to submit a tool result.")
        return self.submit_tool_results(
            [{"name": tool_name, "call_id": call_id, "result": result}],
            interaction_id,
            tools,
        )

    def submit_tool_results(
        self,
        results: list[dict[str, Any]],
        interaction_id: str | None,
        tools: list[dict[str, Any]],
    ) -> ModelReply:
        if not interaction_id or not results or any(not item.get("call_id") for item in results):
            raise RuntimeError("Gemini did not return the identifiers required to submit tool results.")
        interaction = self._get_client().interactions.create(
            model=self.model,
            previous_interaction_id=interaction_id,
            system_instruction=SYSTEM_PROMPT,
            tools=tools,
            input=[
                {
                    "type": "function_result",
                    "name": item["name"],
                    "call_id": item["call_id"],
                    "result": [{"type": "text", "text": json.dumps(item["result"], default=str)}],
                }
                for item in results
            ],
        )
        return self._as_reply(interaction)

    @staticmethod
    def _as_reply(interaction: Any) -> ModelReply:
        calls: list[ToolCall] = []
        call_id: str | None = None
        for step in getattr(interaction, "steps", []) or []:
            if getattr(step, "type", None) != "function_call":
                continue
            calls.append(ToolCall(name=step.name, arguments=step.arguments or {}, call_id=getattr(step, "id", None)))
            call_id = getattr(step, "id", None)
        return ModelReply(
            text=getattr(interaction, "output_text", "") or "",
            tool_calls=calls,
            interaction_id=getattr(interaction, "id", None),
            call_id=call_id,
        )


class MistralChatModel:
    """High-speed Mistral AI model provider using persistent HTTP/2 connection pooling."""

    def __init__(
        self,
        api_key: str | None = None,
        model: str | None = None,
        base_url: str | None = None,
    ) -> None:
        self.api_key = api_key if api_key is not None else MISTRAL_API_KEY
        self.model = model or MISTRAL_MODEL or "mistral-small-latest"
        self.base_url = (base_url or MISTRAL_BASE_URL or "https://api.mistral.ai/v1").rstrip("/")
        self._client: httpx.Client | None = None
        self._sessions: dict[str, list[dict[str, Any]]] = {}

    def _get_client(self) -> httpx.Client:
        if not self.api_key:
            from .config import BACKEND_DIR, ROOT_DIR
            from dotenv import dotenv_values
            env_vals = {**dotenv_values(ROOT_DIR / ".env"), **dotenv_values(BACKEND_DIR / ".env")}
            self.api_key = env_vals.get("MISTRAL_API_KEY", "").strip() or os.getenv("MISTRAL_API_KEY", "").strip()
        if not self.api_key:
            raise RuntimeError(
                "Mistral API key is not configured. Please paste your MISTRAL_API_KEY into backend/.env."
            )
        if self._client is None or self._client.is_closed or self._client.headers.get("Authorization") != f"Bearer {self.api_key}":
            self._client = httpx.Client(
                base_url=self.base_url,
                timeout=25.0,
                headers={
                    "Authorization": f"Bearer {self.api_key}",
                    "Content-Type": "application/json",
                },
                limits=httpx.Limits(max_keepalive_connections=5, max_connections=10),
            )
        return self._client

    def complete(
        self,
        message: str,
        tools: list[dict[str, Any]],
        previous_interaction_id: str | None,
        knowledge_context: list[dict[str, str]],
    ) -> ModelReply:
        client = self._get_client()
        enriched_message = message
        if knowledge_context:
            sources = "\n\n".join(
                f"[{item['source']}]\n{item['content']}" for item in knowledge_context
            )
            enriched_message = (
                "Retrieved general Vault documentation follows. It is not live state; "
                f"use tools for operational facts.\n{sources}\n\nUser request: {message}"
            )

        session_id = previous_interaction_id or uuid4().hex
        messages = list(self._sessions.get(session_id, []))
        if not messages:
            messages.append({"role": "system", "content": SYSTEM_PROMPT})
        messages.append({"role": "user", "content": enriched_message})

        mistral_tools = self._format_tools(tools)
        payload: dict[str, Any] = {
            "model": self.model,
            "messages": messages,
            "temperature": 0.15,
            "max_tokens": 1200,
        }
        if mistral_tools:
            payload["tools"] = mistral_tools
            payload["tool_choice"] = "auto"

        response = client.post("/chat/completions", json=payload)
        return self._handle_response(response, session_id, messages)

    def submit_tool_result(
        self,
        tool_name: str,
        call_id: str | None,
        result: dict[str, Any],
        interaction_id: str | None,
        tools: list[dict[str, Any]],
    ) -> ModelReply:
        if not call_id:
            raise RuntimeError("Mistral requires a call ID to submit tool result.")
        return self.submit_tool_results(
            [{"name": tool_name, "call_id": call_id, "result": result}],
            interaction_id,
            tools,
        )

    def submit_tool_results(
        self,
        results: list[dict[str, Any]],
        interaction_id: str | None,
        tools: list[dict[str, Any]],
    ) -> ModelReply:
        if not interaction_id or interaction_id not in self._sessions:
            raise RuntimeError("Mistral session interaction not found to submit tool results.")

        client = self._get_client()
        messages = self._sessions[interaction_id]
        for item in results:
            messages.append({
                "role": "tool",
                "tool_call_id": item["call_id"],
                "name": item["name"],
                "content": json.dumps(item["result"], default=str),
            })

        mistral_tools = self._format_tools(tools)
        payload: dict[str, Any] = {
            "model": self.model,
            "messages": messages,
            "temperature": 0.15,
            "max_tokens": 1200,
        }
        if mistral_tools:
            payload["tools"] = mistral_tools
            payload["tool_choice"] = "auto"

        response = client.post("/chat/completions", json=payload)
        return self._handle_response(response, interaction_id, messages)

    @staticmethod
    def _format_tools(tools: list[dict[str, Any]]) -> list[dict[str, Any]]:
        mistral_tools = []
        for t in tools:
            if "function" in t:
                mistral_tools.append(t)
            elif t.get("type") == "function":
                mistral_tools.append({
                    "type": "function",
                    "function": {
                        "name": t.get("name"),
                        "description": t.get("description", ""),
                        "parameters": t.get("parameters", {}),
                    },
                })
        return mistral_tools

    def _handle_response(
        self, response: httpx.Response, session_id: str, messages: list[dict[str, Any]]
    ) -> ModelReply:
        if response.status_code != 200:
            err_text = response.text
            try:
                err_data = response.json()
                err_text = (
                    err_data.get("message")
                    or err_data.get("error", {}).get("message")
                    or err_text
                )
            except Exception:
                pass
            raise RuntimeError(f"Mistral API error ({response.status_code}): {err_text}")

        data = response.json()
        choice = data["choices"][0]
        msg = choice["message"]
        messages.append(msg)
        self._sessions[session_id] = messages

        tool_calls: list[ToolCall] = []
        for tc in msg.get("tool_calls") or []:
            fn = tc.get("function", {})
            args = fn.get("arguments", "{}")
            if isinstance(args, str):
                try:
                    args = json.loads(args)
                except Exception:
                    args = {}
            tool_calls.append(
                ToolCall(
                    name=fn.get("name", ""),
                    arguments=args,
                    call_id=tc.get("id"),
                )
            )

        return ModelReply(
            text=msg.get("content") or "",
            tool_calls=tool_calls if tool_calls else None,
            interaction_id=session_id,
            call_id=tool_calls[0].call_id if tool_calls else None,
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
        max_tool_iterations: int = DEFAULT_MAX_TOOL_ITERATIONS,
    ) -> None:
        self.service = service or MockVaultService()
        self.tools = ToolRegistry(self.service)
        if model is not None:
            self.model = model
        elif LLM_PROVIDER == "mistral" or MISTRAL_API_KEY:
            self.model = MistralChatModel()
        else:
            self.model = GeminiInteractionsModel()
        self.audit_log = audit_log or AuditLog()
        self.retriever = retriever or KnowledgeRetriever()
        self.approval_ttl_seconds = approval_ttl_seconds
        self.max_tool_iterations = max(1, max_tool_iterations)
        self.pending_actions: dict[str, PendingAction] = {}
        self.session_interactions: dict[str, str] = {}
        self._lock = RLock()

    def chat(self, message: str, session_id: str | None = None) -> dict[str, Any]:
        session_id = session_id or uuid4().hex
        grounded_by_tool = False
        tool_calls_processed = 0
        provider_name = "mistral" if isinstance(self.model, MistralChatModel) else "gemini"
        tools_list = self.tools.declarations(provider=provider_name)
        self.audit_log.record(session_id=session_id, user_request=message, status="received")
        try:
            reply = self.model.complete(
                message=message,
                tools=tools_list,
                previous_interaction_id=self.session_interactions.get(session_id),
                knowledge_context=self.retriever.retrieve(message),
            )
            for _ in range(self.max_tool_iterations):
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
                validated_calls: list[tuple[ToolCall, Any, dict[str, Any], str]] = []
                for call in calls:
                    tool = self.tools.get(call.name)
                    arguments = self.tools.validate_arguments(tool, call.arguments)
                    call_id = call.call_id or (reply.call_id if len(calls) == 1 else None)
                    if not call_id:
                        raise RuntimeError("Gemini returned a function call without a call ID.")
                    validated_calls.append((call, tool, arguments, call_id))

                if tool_calls_processed + len(validated_calls) > self.max_tool_iterations:
                    return self._error("The request needed too many consecutive tool calls. Please make it more specific.", session_id)
                tool_calls_processed += len(validated_calls)

                state_changing = next(
                    (item for item in validated_calls if requires_approval(item[1].risk_level)),
                    None,
                )
                if state_changing:
                    _, tool, arguments, call_id = state_changing
                    pending = self._create_pending_action(
                        tool.name,
                        arguments,
                        tool.risk_level,
                        tool.reason,
                        session_id,
                        reply.interaction_id,
                        call_id,
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

                tool_results: list[dict[str, Any]] = []
                for _, tool, arguments, call_id in validated_calls:
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
                    tool_results.append({"name": tool.name, "call_id": call_id, "result": result})

                if len(tool_results) == 1:
                    item = tool_results[0]
                    reply = self.model.submit_tool_result(
                        tool_name=item["name"],
                        call_id=item["call_id"],
                        result=item["result"],
                        interaction_id=reply.interaction_id,
                        tools=tools_list,
                    )
                else:
                    reply = self.model.submit_tool_results(
                        results=tool_results,
                        interaction_id=reply.interaction_id,
                        tools=tools_list,
                    )
            return self._error("The request needed too many consecutive tool calls. Please make it more specific.", session_id)
        except LookupError as exc:
            self.audit_log.record(session_id=session_id, user_request=message, status="unknown_tool", error=str(exc))
            return self._error("I couldn't match that request to an available Vault operation.", session_id)
        except ToolArgumentError as exc:
            self.audit_log.record(session_id=session_id, user_request=message, status="invalid_arguments", error=str(exc))
            return self._error(str(exc), session_id)
        except Exception as exc:
            logger.exception("Vault AI chat failed for session %s", session_id)
            err_msg = str(exc)
            if "RateLimitError" in err_msg or "rate limit" in err_msg.lower() or "429" in err_msg:
                p_name = "Mistral" if isinstance(self.model, MistralChatModel) else "AI"
                user_msg = f"{p_name} API rate limit reached. Please wait a moment before trying again."
            else:
                user_msg = "Vault AI could not complete the request. Check the backend configuration and try again."
            self.audit_log.record(session_id=session_id, user_request=message, status="failed", error=err_msg)
            return self._error(user_msg, session_id)

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
            verification_succeeded = True
            if tool.name == "repair_replica":
                verification = self._verify_repair(arguments["object_id"])
                result = {**result, "verification": verification}
                verification_succeeded = verification["success"]
            final_status = ActionStatus.EXECUTED if verification_succeeded else ActionStatus.FAILED
            self._update_action(approved.model_copy(update={"status": final_status}))
            execution_status = "SUCCESS" if verification_succeeded else "VERIFICATION_FAILED"
            self.audit_log.record(action_id=action_id, tool=tool.name, approval_status="APPROVED", execution_status=execution_status, result_summary=result)
            message = self._repair_message(result) if tool.name == "repair_replica" else self._success_message(tool.name, result)
            try:
                reply = self.model.submit_tool_result(
                    tool_name=tool.name,
                    call_id=approved.call_id,
                    result=result,
                    interaction_id=approved.interaction_id,
                    tools=self.tools.declarations(),
                )
                self._remember(approved.session_id or "", reply.interaction_id)
                if tool.name != "repair_replica" or verification_succeeded:
                    message = reply.text.strip() or message
            except Exception:
                logger.exception("Vault operation succeeded but Gemini could not format the result")
            return {
                "type": "tool_result",
                "success": verification_succeeded,
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

    def _verify_repair(self, object_id: str) -> dict[str, Any]:
        metadata = self.tools.validate_and_execute("get_object_metadata", {"object_id": object_id})
        integrity = self.tools.validate_and_execute("verify_integrity", {"object_id": object_id})
        required = metadata["replication_factor"]
        healthy = metadata["healthy_replica_count"]
        success = (
            metadata["status"] == "healthy"
            and healthy >= required
            and integrity["integrity_status"] == "healthy"
            and not integrity["corrupted_replicas"]
        )
        return {
            "success": success,
            "object_id": object_id,
            "status": metadata["status"],
            "healthy_replica_count": healthy,
            "required_replica_count": required,
            "replica_locations": metadata["active_replica_locations"],
            "integrity_status": integrity["integrity_status"],
            "missing_replica_count": metadata["missing_replica_count"],
        }

    @staticmethod
    def _repair_message(result: dict[str, Any]) -> str:
        verification = result["verification"]
        if not verification["success"]:
            return (
                f"Repair was attempted for {verification['object_id']}, but verification found "
                f"{verification['healthy_replica_count']} of {verification['required_replica_count']} "
                "healthy replicas. The object remains degraded."
            )
        return (
            f"Replica repair verified: {verification['object_id']} is healthy with "
            f"{verification['healthy_replica_count']} of {verification['required_replica_count']} "
            f"required replicas; integrity is {verification['integrity_status']}."
        )

    @staticmethod
    def _error(message: str, session_id: str) -> dict[str, Any]:
        return {"type": "error", "message": message, "session_id": session_id}

    @staticmethod
    def _is_educational_or_general(message: str) -> bool:
        normalized = message.strip().lower()
        if not normalized:
            return True
        educational_cues = (
            "explain", "how does", "how do", "how can", "how to", "why does", "why do",
            "why is", "why are", "why did", "what is", "what are", "what does", "what happens",
            "teach me", "tell me about", "describe", "difference between", "compare",
            "can you explain", "could you explain", "help me understand", "walk me through",
            "guide me", "under the hood", "in simple terms", "concept", "theory",
            "how it works", "architecture", "overview", "principle", "algorithm",
        )
        conversational_cues = (
            "hello", "hi", "hey", "who are you", "what can you do", "thanks", "thank you",
            "good morning", "good evening", "howdy", "greetings", "help",
        )
        if any(cue in normalized for cue in educational_cues):
            return True
        if any(
            normalized == cue
            or normalized.startswith(cue + " ")
            or normalized.startswith(cue + ",")
            or normalized.startswith(cue + "!")
            or normalized.startswith(cue + "?")
            for cue in conversational_cues
        ):
            return True
        return False

    @staticmethod
    def _requires_live_tool(message: str) -> bool:
        if VaultAIAgent._is_educational_or_general(message):
            return False
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