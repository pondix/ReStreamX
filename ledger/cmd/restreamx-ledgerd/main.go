package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"restreamx/ledger/internal/raft"
	"restreamx/ledger/internal/store"
	"restreamx/pkg/api"
)

type server struct {
	selfAddr string
	quorum   *raft.Quorum
	store    *store.Store
	mu       sync.Mutex
}

func newServer(self string, peers []string, st *store.Store) *server {
	return &server{
		selfAddr: self,
		quorum:   &raft.Quorum{LeaderAddr: self, Peers: peers, Timeout: 2 * time.Second},
		store:    st,
	}
}

func (s *server) acquireLease(w http.ResponseWriter, r *http.Request) {
	var req api.AcquireLeaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !s.quorum.IsLeader(s.selfAddr) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("not leader"))
		return
	}
	lease := &api.Lease{RangeId: req.RangeId, OwnerId: req.OwnerId, Epoch: uint64(time.Now().UnixNano()), ExpiryMs: time.Now().Add(time.Duration(req.TtlMs) * time.Millisecond).UnixMilli()}
	if err := s.store.PutLease(lease); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := s.quorum.ReplicateLease(r.Context(), lease); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_ = json.NewEncoder(w).Encode(lease)
}

func (s *server) renewLease(w http.ResponseWriter, r *http.Request) {
	var req api.RenewLeaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !raft.ReplicationHeader(r) && !s.quorum.IsLeader(s.selfAddr) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("not leader"))
		return
	}
	lease := &api.Lease{RangeId: req.RangeId, OwnerId: req.OwnerId, Epoch: req.Epoch, ExpiryMs: time.Now().Add(time.Duration(req.TtlMs) * time.Millisecond).UnixMilli()}
	if err := s.store.PutLease(lease); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !raft.ReplicationHeader(r) {
		if err := s.quorum.ReplicateLease(r.Context(), lease); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
	}
	_ = json.NewEncoder(w).Encode(lease)
}

func (s *server) getLease(w http.ResponseWriter, r *http.Request) {
	rangeID := r.URL.Query().Get("range_id")
	if rangeID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	lease, err := s.store.GetLease(rangeID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(lease)
}

func (s *server) appendSegment(w http.ResponseWriter, r *http.Request) {
	var seg api.Segment
	if err := json.NewDecoder(r.Body).Decode(&seg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !raft.ReplicationHeader(r) && !s.quorum.IsLeader(s.selfAddr) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("not leader"))
		return
	}
	idx, err := s.store.NextCommitIndex()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	seg.CommitIndex = idx
	if err := s.store.PutSegment(&seg); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !raft.ReplicationHeader(r) {
		if err := s.quorum.ReplicateSegment(r.Context(), &seg); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
	}
	_ = json.NewEncoder(w).Encode(api.AppendSegmentResponse{CommitIndex: idx})
}

func (s *server) subscribe(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from_commit_index")
	var start uint64
	if from != "" {
		_, _ = fmt.Sscanf(from, "%d", &start)
	}
	segs, err := s.store.ListSegments(start)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(segs)
}

func (s *server) status(w http.ResponseWriter, r *http.Request) {
	idx, err := s.store.GetCommitIndex()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(api.StatusResponse{Leader: s.quorum.LeaderAddr, Term: 1, CommitIndex: idx, Peers: s.quorum.Peers})
}

func metricsHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idx, _ := st.GetCommitIndex()
		_, _ = fmt.Fprintf(w, "ledger_commit_index %d\n", idx)
	}
}

func main() {
	var (
		listen  = flag.String("listen", ":7000", "listen address")
		data    = flag.String("data", "/var/lib/restreamx/ledger.json", "data path")
		peers   = flag.String("peers", "", "comma peers addresses")
		metrics = flag.String("metrics", ":7001", "metrics listen")
		leader  = flag.String("leader", "", "leader address")
	)
	flag.Parse()
	if err := os.MkdirAll("/var/lib/restreamx", 0755); err != nil && !os.IsExist(err) {
		log.Printf("data dir: %v", err)
	}
	st, err := store.Open(*data)
	if err != nil {
		log.Fatalf("store open: %v", err)
	}
	defer st.Close()
	peerList := []string{}
	if *peers != "" {
		peerList = strings.Split(*peers, ",")
	}
	self := *leader
	if self == "" {
		self = *listen
	}
	srv := newServer(self, peerList, st)
	if *leader != "" {
		srv.quorum.LeaderAddr = *leader
	} else {
		srv.quorum.LeaderAddr = *listen
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/lease/acquire", srv.acquireLease)
	mux.HandleFunc("/lease/renew", srv.renewLease)
	mux.HandleFunc("/lease/get", srv.getLease)
	mux.HandleFunc("/segment/append", srv.appendSegment)
	mux.HandleFunc("/segment/subscribe", srv.subscribe)
	mux.HandleFunc("/status", srv.status)

	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", metricsHandler(st))

	server := &http.Server{Addr: *listen, Handler: mux}
	metricsServer := &http.Server{Addr: *metrics, Handler: metricsMux}

	go func() {
		log.Printf("metrics listening %s", *metrics)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("metrics: %v", err)
		}
	}()
	go func() {
		log.Printf("ledger listening %s", *listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	_ = metricsServer.Shutdown(ctx)
	fmt.Println("ledger stopped")
}
