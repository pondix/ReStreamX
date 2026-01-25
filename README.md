# ReStreamX

ReStreamX is a ledger-backed, lease-fenced replication system for MySQL 8.4. It includes a MySQL plugin, a ledger quorum, stateless routers, and per-MySQL agents.

## Quick start
```
make up
make e2e
```

## Repo layout
- `ledger/`: restreamx-ledgerd quorum log
- `router/`: write router
- `agent/`: apply agent
- `mysql-plugin/`: MySQL plugin source and packaging
- `deploy/`: docker compose and scripts
- `docs/`: architecture and ops

See `docs/` for details.
