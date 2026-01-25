package api

type Lease struct {
	RangeId  string `json:"range_id"`
	OwnerId  string `json:"owner_id"`
	Epoch    uint64 `json:"epoch"`
	ExpiryMs int64  `json:"expiry_ms"`
}

type Segment struct {
	RangeId      string `json:"range_id"`
	Epoch        uint64 `json:"epoch"`
	TxnId        string `json:"txn_id"`
	CommitIndex  uint64 `json:"commit_index"`
	PayloadType  string `json:"payload_type"`
	PayloadBytes []byte `json:"payload_bytes"`
	Checksum     uint32 `json:"checksum"`
}

type AcquireLeaseRequest struct {
	RangeId string `json:"range_id"`
	OwnerId string `json:"owner_id"`
	TtlMs   int64  `json:"ttl_ms"`
}

type RenewLeaseRequest struct {
	RangeId string `json:"range_id"`
	OwnerId string `json:"owner_id"`
	Epoch   uint64 `json:"epoch"`
	TtlMs   int64  `json:"ttl_ms"`
}

type GetLeaseRequest struct {
	RangeId string `json:"range_id"`
}

type AppendSegmentResponse struct {
	CommitIndex uint64 `json:"commit_index"`
}

type SubscribeRequest struct {
	FromCommitIndex uint64 `json:"from_commit_index"`
}

type StatusResponse struct {
	Leader      string   `json:"leader"`
	Term        uint64   `json:"term"`
	CommitIndex uint64   `json:"commit_index"`
	Peers       []string `json:"peers"`
}
