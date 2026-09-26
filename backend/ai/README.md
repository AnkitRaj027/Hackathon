# VAULT AI Control Layer

The AI layer translates requests into registered Vault operations. Gemini may request a typed function call; application code validates its name and arguments, applies permission policy, and invokes the VaultService adapter. Gemini never executes Python or accesses storage directly.

## Architecture and Workflow

`POST /api/ai/chat` passes the message and optional session ID to `VaultAIAgent`. The agent supplies the explicit tool declarations and system policy to Google's `google-genai` Interactions API. Read calls are validated and executed by the backend; their results are returned to Gemini as a function result for a final explanation. Modifying calls create a server-side pending action and stop before execution. The approval endpoint looks up that stored action, revalidates it, and executes its exact tool and arguments.

The tool registry in `tools.py` contains each function schema, implementation, risk level, and approval policy. Do not add a direct execution path around `ToolRegistry` or `VaultAIAgent.approve`.

## Permissions and Approvals

Read-only operations are low risk and execute immediately. Repairs, rebalancing, uploads, node simulation/restoration, and replication changes require medium-risk approval. Object deletion and node removal require high-risk approval with a warning.

Pending actions use cryptographically random IDs and include immutable action fields, status, creation time, and expiry. Approval input contains only `approved`; tool names and arguments are never accepted from the frontend. Duplicate approvals conflict, expired actions return HTTP 410, and actions are held in memory for this hackathon version. Use a durable transactional store and authenticated user authorization before production deployment.

## VaultService Integration

`services/vault_service.py` defines the integration protocol. `MockVaultService` starts with four healthy nodes and `report.pdf` replicated on node1-node3; it supports health changes, metadata, integrity, repair, rebalancing, replication updates, upload registration, object deletion, and node removal.

To integrate the real backend, implement the `VaultService` methods in an adapter, map its native errors to clear exceptions, and inject it into `VaultAIAgent(service=RealVaultService(...))` in `backend/main.py`. Keep authorization in the shared AI router/agent and do not expose raw database, shell, or filesystem methods. `upload_object` accepts an opaque backend-owned file reference, not an arbitrary local path; connect it to the real upload workflow when that contract is available.

## Gemini Configuration

The SDK is `google-genai`, imported as `from google import genai`. The client is created lazily on the first model request so the API can start and unit tests can run without a key. Copy `backend/.env.example` to `backend/.env`, provide a valid `GEMINI_API_KEY`, and adjust `GEMINI_MODEL` if needed. The key is only read by the backend and must not be sent to a browser or committed. The implementation uses the Interactions API and explicit application-side tool execution, not `google.generativeai` or automatic code execution.

## RAG and MCP

`rag/retriever.py` is a replaceable, local keyword retriever for the included Markdown knowledge notes. Retrieved notes provide generic explanations only; the system prompt requires live tool results for current node/object facts. Replace the retriever behind its current interface if a vector store becomes available.

The registry schemas and VaultService protocol are the MCP extension point. Any future MCP transport must call the same policy-enforcing agent and must not execute tools outside the approval flow.

## API

- `POST /api/ai/chat` with `{"message":"Show node health","session_id":"optional-session"}` returns `answer`, `approval_required`, or `error`.
- `POST /api/ai/approve/{action_id}` with `{"approved":true}` executes the stored action.
- `POST /api/ai/reject/{action_id}` rejects a pending action without execution.
- `GET /api/ai/actions` lists pending and recent in-memory actions.
- `GET /api/ai/health` reports service and model configuration status without exposing the key.

CORS allows only `FRONTEND_ORIGIN` (default `http://localhost:3000`). Set it explicitly for the frontend origin. No frontend exists in the inspected repository, so this backend does not create a parallel UI.

## Run and Test

From the repository root:

```powershell
py -m venv .venv
.venv\Scripts\Activate.ps1
pip install -r backend\requirements.txt
if (-not (Test-Path backend\.env)) { Copy-Item backend\.env.example backend\.env }
uvicorn backend.main:app --reload
```

Run automated tests with:

```powershell
pytest backend\ai\tests -q
```

Example read request:

```powershell
curl.exe -X POST http://127.0.0.1:8000/api/ai/chat -H "Content-Type: application/json" -d '{"message":"Show me the health of all storage nodes."}'
```

For a mutating request, retain the returned `action_id` and approve or reject it explicitly:

```powershell
curl.exe -X POST http://127.0.0.1:8000/api/ai/approve/ACTION_ID -H "Content-Type: application/json" -d '{"approved":true}'
curl.exe -X POST http://127.0.0.1:8000/api/ai/reject/ACTION_ID
```

Demo prompts include node health, unavailable nodes, `report.pdf` metadata/replica count/integrity, replica repair, rebalance, node2 failure and restore, replication factor update, and deletion. A configured valid Gemini key is required for natural-language API requests. Unit tests fake the model and do not call Gemini.

## Replica Repair Demo

Open `http://127.0.0.1:8000/docs` after starting the API. In Swagger, use `POST /api/ai/chat` to ask `Simulate failure of node3`; approve the returned action with `POST /api/ai/approve/{action_id}` and `{"approved":true}`. Then ask `Show metadata for object-1` to inspect its two active replicas and degraded status. Ask `Repair the missing replica of object-1`; the chat response must be `approval_required`. Approve that action. The result includes a fresh metadata and integrity verification, with three healthy replicas and healthy integrity. Repair is never run by the chat request itself.

For multi-tool read checks, submit `Show me the replica status of object-1` or `Check object-1 and tell me whether it needs repair` to `POST /api/ai/chat`. The agent validates and executes read-only calls, returns their results to Gemini, and continues until it gets a final answer or reaches its tool-call limit. If Gemini requests `repair_replica`, the agent stops and returns `approval_required`; it never executes a modifying call from a read batch.

## Audit and Operational Limits

Structured audit events are written to the server logger and held in memory for development. The current pending-action store, session interaction IDs, and audit event list are process-local and are lost on restart. Add persistent storage, authenticated principals, rate limiting, and request-level authorization before production use. API responses intentionally omit stack traces; details are logged server-side.