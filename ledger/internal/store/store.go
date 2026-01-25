package store

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"restreamx/pkg/api"
)

type snapshot struct {
	CommitIndex uint64                `json:"commit_index"`
	Leases      map[string]*api.Lease `json:"leases"`
	Segments    []*api.Segment        `json:"segments"`
}

type Store struct {
	mu          sync.Mutex
	path        string
	commitIndex uint64
	leases      map[string]*api.Lease
	segments    []*api.Segment
}

func Open(path string) (*Store, error) {
	st := &Store{path: path, leases: map[string]*api.Lease{}, segments: []*api.Segment{}}
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var snap snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			return nil, err
		}
		st.commitIndex = snap.CommitIndex
		st.leases = snap.Leases
		st.segments = snap.Segments
	}
	return st, nil
}

func (s *Store) Close() error { return nil }

func (s *Store) persist() error {
	snap := snapshot{CommitIndex: s.commitIndex, Leases: s.leases, Segments: s.segments}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) GetCommitIndex() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commitIndex, nil
}

func (s *Store) NextCommitIndex() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commitIndex++
	if err := s.persist(); err != nil {
		return 0, err
	}
	return s.commitIndex, nil
}

func (s *Store) PutLease(lease *api.Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leases[lease.RangeId] = lease
	return s.persist()
}

func (s *Store) GetLease(rangeID string) (*api.Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lease, ok := s.leases[rangeID]
	if !ok {
		return nil, errors.New("lease not found")
	}
	return lease, nil
}

func (s *Store) PutSegment(seg *api.Segment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.segments = append(s.segments, seg)
	return s.persist()
}

func (s *Store) ListSegments(from uint64) ([]*api.Segment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*api.Segment{}
	for _, seg := range s.segments {
		if seg.CommitIndex >= from {
			out = append(out, seg)
		}
	}
	return out, nil
}
