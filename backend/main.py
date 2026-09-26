import os
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from backend.ai.agent import VaultAIAgent
from backend.ai.config import FRONTEND_ORIGINS, VAULT_BACKEND_URL
from backend.ai.router import router as ai_router
from backend.ai.services.http_vault_service import HttpVaultService
from backend.ai.services.mock_vault_service import MockVaultService


def create_app(agent: VaultAIAgent | None = None) -> FastAPI:
    application = FastAPI(title="VAULT AI Control API", version="0.1.0")
    if agent is None:
        service_type = os.getenv("VAULT_SERVICE_TYPE", "live").lower()
        service = HttpVaultService(base_url=VAULT_BACKEND_URL) if service_type == "live" else MockVaultService()
        agent = VaultAIAgent(service=service)
    application.state.vault_ai_agent = agent
    application.add_middleware(
        CORSMiddleware,
        allow_origins=FRONTEND_ORIGINS,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    application.include_router(ai_router)
    return application


app = create_app()