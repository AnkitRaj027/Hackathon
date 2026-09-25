import os
from pathlib import Path

from dotenv import load_dotenv


BACKEND_DIR = Path(__file__).resolve().parents[1]
load_dotenv(BACKEND_DIR / ".env")

GEMINI_API_KEY = os.getenv("GEMINI_API_KEY", "").strip()
GEMINI_MODEL = os.getenv("GEMINI_MODEL", "gemini-3.8-flash").strip()
FRONTEND_ORIGIN = os.getenv("FRONTEND_ORIGIN", "http://localhost:3000").strip()
VAULT_BACKEND_URL = os.getenv("VAULT_BACKEND_URL", "http://localhost:8000").strip()
APPROVAL_TTL_SECONDS = max(30, int(os.getenv("APPROVAL_TTL_SECONDS", "300")))