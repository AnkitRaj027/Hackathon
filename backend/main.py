from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from backend.ai.agent import VaultAIAgent
from backend.ai.config import FRONTEND_ORIGIN
from backend.ai.router import router as ai_router


def create_app(agent: VaultAIAgent | None = None) -> FastAPI:
    application = FastAPI(title="VAULT AI Control API", version="0.1.0")
    application.state.vault_ai_agent = agent or VaultAIAgent()
    application.add_middleware(
        CORSMiddleware,
        allow_origins=[FRONTEND_ORIGIN],
        allow_credentials=True,
        allow_methods=["GET", "POST"],
        allow_headers=["Content-Type", "Authorization"],
    )
    application.include_router(ai_router)
    return application


app = create_app()