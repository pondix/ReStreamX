package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"restreamx/pkg/api"
)

const replicateHeader = "X-RestreamX-Replicate"

var ErrNotLeader = errors.New("not leader")

type Quorum struct {
	LeaderAddr string
	Peers      []string
	Timeout    time.Duration
}

func (q *Quorum) IsLeader(self string) bool {
	return self == q.LeaderAddr
}

func (q *Quorum) ReplicateSegment(ctx context.Context, seg *api.Segment) error {
	return q.replicate(ctx, "/segment/append", seg)
}

func (q *Quorum) ReplicateLease(ctx context.Context, lease *api.Lease) error {
	return q.replicate(ctx, "/lease/renew", &api.RenewLeaseRequest{
		RangeId: lease.RangeId,
		OwnerId: lease.OwnerId,
		Epoch:   lease.Epoch,
		TtlMs:   lease.ExpiryMs,
	})
}

func (q *Quorum) replicate(ctx context.Context, path string, payload any) error {
	if len(q.Peers) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, q.Timeout)
	defer cancel()
	acks := 1
	needed := (len(q.Peers)+1)/2 + 1
	for _, peer := range q.Peers {
		if peer == q.LeaderAddr {
			continue
		}
		if err := post(ctx, peer, path, payload); err == nil {
			acks++
		}
		if acks >= needed {
			return nil
		}
	}
	return errors.New("failed to reach quorum")
}

func post(ctx context.Context, base, path string, payload any) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+base+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(replicateHeader, "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return errors.New("replication failed")
	}
	return nil
}

func ReplicationHeader(r *http.Request) bool {
	return r.Header.Get(replicateHeader) == "true"
}
