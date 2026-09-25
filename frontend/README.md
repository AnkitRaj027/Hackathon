# Vault Frontend (Phase 2+)

> **Phase 1 Status:** As specified in Section 33 of the Vault Engineering Requirements, the 3D dashboard and web frontend are scheduled for Phase 2 once the distributed storage core, metadata consensus, and replication invariants are proven.

## Planned Architecture
- **Framework**: React 18 + TypeScript + Vite
- **3D Engine**: Three.js via `@react-three/fiber` & `@react-three/drei`
- **Data Source**: Live WebSocket streaming directly from Vault Coordinator & Metadata Service (no fake/simulated state)
- **Live Invariants**: Node health states (`HEALTHY -> SUSPECT -> DEAD`), real chunk placements, and visual repair particle flows driven by actual backend events.
