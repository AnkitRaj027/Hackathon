# VAULT Architecture

Vault stores objects across multiple storage nodes. The AI control layer is an adapter and must not own storage, metadata, or replica state. It may only call registered VaultService methods, and current operational questions must be answered from those methods.