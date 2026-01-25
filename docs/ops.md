# Ops

## Bringing up the stack
```
make up
```

## Failover
1. Call router admin endpoint to acquire a lease for a different owner.
2. Router updates MySQL plugin modes across nodes.
3. Wait for agents to apply segments and verify counts.

## Debugging
- Ledger status: `curl http://ledger1:7000/status`.
- Router health: `curl http://router1:8080/metrics`.
- Agent health: `curl http://agent1:9090/metrics`.
