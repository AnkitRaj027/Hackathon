from fastapi import APIRouter, HTTPException, Request

from .agent import ActionError, VaultAIAgent
from .models import ApprovalRequest, ChatRequest


router = APIRouter(prefix="/api/ai", tags=["Vault AI"])


def _agent(request: Request) -> VaultAIAgent:
    return request.app.state.vault_ai_agent


@router.post("/chat")
def chat(payload: ChatRequest, request: Request) -> dict:
    return _agent(request).chat(payload.message, payload.session_id)


@router.post("/approve/{action_id}")
def approve(action_id: str, payload: ApprovalRequest, request: Request) -> dict:
    if not payload.approved:
        raise HTTPException(status_code=400, detail="Use the reject endpoint to cancel a pending action.")
    try:
        return _agent(request).approve(action_id)
    except ActionError as exc:
        raise HTTPException(status_code=exc.status_code, detail=str(exc)) from exc


@router.post("/reject/{action_id}")
def reject(action_id: str, request: Request) -> dict:
    try:
        return _agent(request).reject(action_id)
    except ActionError as exc:
        raise HTTPException(status_code=exc.status_code, detail=str(exc)) from exc


@router.get("/actions")
def actions(request: Request) -> dict:
    return {"actions": _agent(request).list_actions()}


@router.get("/health")
def health(request: Request) -> dict:
    agent = _agent(request)
    provider = "mistral" if type(agent.model).__name__ == "MistralChatModel" else "gemini"
    return {
        "status": "ok",
        "provider": provider,
        "model_configured": bool(getattr(agent.model, "api_key", None)),
        "model": getattr(agent.model, "model", "injected"),
        "service": type(agent.service).__name__,
    }