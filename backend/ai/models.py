from datetime import datetime
from typing import Any

from pydantic import BaseModel, ConfigDict, Field

from .permissions import ActionStatus, RiskLevel


class ChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=8000)
    session_id: str | None = Field(default=None, max_length=128)


class ApprovalRequest(BaseModel):
    approved: bool


class PendingAction(BaseModel):
    model_config = ConfigDict(frozen=True)

    action_id: str
    tool_name: str
    arguments: dict[str, Any]
    risk_level: RiskLevel
    reason: str
    created_at: datetime
    expires_at: datetime
    status: ActionStatus = ActionStatus.PENDING
    session_id: str | None = None
    interaction_id: str | None = None
    call_id: str | None = None


class ToolCall(BaseModel):
    name: str
    arguments: dict[str, Any] = Field(default_factory=dict)
    call_id: str | None = None


class AIResponse(BaseModel):
    type: str
    message: str | None = None
    action_id: str | None = None
    tool: str | None = None
    arguments: dict[str, Any] | None = None
    risk_level: str | None = None
    reason: str | None = None
    success: bool | None = None
    result: Any | None = None
