# AI Permissions

Read-only queries do not require approval. Repairs, rebalancing, node simulation or restoration, uploads, and replication changes require explicit approval. Object deletion and node removal are destructive and require high-risk approval. The backend is the authority for pending actions and executes only the arguments stored with the pending action.