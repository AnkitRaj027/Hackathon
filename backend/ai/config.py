import os
from pathlib import Path

from dotenv import load_dotenv


BACKEND_DIR = Path(__file__).resolve().parents[1]
ROOT_DIR = BACKEND_DIR.parent
load_dotenv(BACKEND_DIR / ".env")
load_dotenv(ROOT_DIR / ".env")

GEMINI_API_KEY = os.getenv("GEMINI_API_KEY", "").strip()
GEMINI_MODEL = os.getenv("GEMINI_MODEL", "gemini-3.8-flash").strip()

MISTRAL_API_KEY = os.getenv("MISTRAL_API_KEY", "").strip()
MISTRAL_MODEL = os.getenv("MISTRAL_MODEL", "mistral-small-latest").strip()
MISTRAL_BASE_URL = os.getenv("MISTRAL_BASE_URL", "https://api.mistral.ai/v1").strip()
LLM_PROVIDER = os.getenv("LLM_PROVIDER", "mistral" if MISTRAL_API_KEY else "gemini").strip().lower()

DEFAULT_ORIGINS = (
    "http://localhost:8085,http://localhost:5173,http://localhost:3000,"
    "http://127.0.0.1:8085,http://127.0.0.1:5173,http://127.0.0.1:3000"
)
raw_origins = os.getenv("FRONTEND_ORIGINS") or os.getenv("FRONTEND_ORIGIN") or DEFAULT_ORIGINS
FRONTEND_ORIGINS = [o.strip() for o in raw_origins.split(",") if o.strip()]
FRONTEND_ORIGIN = FRONTEND_ORIGINS[0] if FRONTEND_ORIGINS else "http://localhost:8085"

VAULT_BACKEND_URL = os.getenv("VAULT_BACKEND_URL", "http://localhost:8085").strip()
APPROVAL_TTL_SECONDS = max(30, int(os.getenv("APPROVAL_TTL_SECONDS", "300")))