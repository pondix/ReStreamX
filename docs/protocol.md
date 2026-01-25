# ReStreamX Protocol

## Lease
```
Lease { range_id, owner_id, epoch, expiry_ms }
```
Epoch increases on lease acquisition. Routers must append segments using the current epoch. Agents reject segments with older epochs for a range.

## Segment
```
Segment { range_id, epoch, txn_id, commit_index, payload_type, payload_bytes, checksum }
```
Segments are ordered by commit_index and applied idempotently. The MVP payload_type is `json` with a single write operation.

## API surface (MVP HTTP/JSON)
- `POST /lease/acquire`
- `POST /lease/renew`
- `GET /lease/get?range_id=...`
- `POST /segment/append`
- `GET /segment/subscribe?from_commit_index=...`
- `GET /status`

## Fencing rules
- Only the lease owner may accept writes for the range (router updates MySQL plugin mode).
- Replica nodes reject user writes in REPLICA mode (apply user is allowed).
- Segments with stale epochs are rejected by agents.
