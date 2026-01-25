# ReStreamX Architecture

ReStreamX is a ledger-backed MySQL replication system built around leases and ordered segments. The router performs writes on the current lease owner, appends a segment to the ledger, and replicas apply the ordered segments to converge. Each MySQL node runs a local agent to stream ledger segments and apply them.

## Components
- **restreamx-ledgerd**: 3-node quorum log with lease and segment APIs over HTTP/JSON.
- **restreamx-router**: stateless write router that appends segments before acknowledging writes.
- **restreamx-agent**: per-MySQL daemon that subscribes to segments and applies them.
- **restreamx plugin**: MySQL audit plugin enforcing write fencing on replicas and exposing status/system variables.

## Data flow
1. Router gets lease for a RangeID.
2. Router writes to lease owner MySQL.
3. Router appends a segment containing the write payload.
4. Agents poll the ledger and apply segments locally.
