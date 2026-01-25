package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"restreamx/pkg/api"
)

type config struct {
	LedgerAddr string
	RangeID    string
	OwnerMap   map[string]string
	Timeout    time.Duration
	MySQLUser  string
	MySQLPass  string
	MySQLDB    string
	AdminUser  string
	AdminPass  string
}

type writeRequest struct {
	Op    string                 `json:"op"`
	Table string                 `json:"table"`
	ID    int                    `json:"id"`
	Data  map[string]interface{} `json:"data"`
}

type router struct {
	cfg        config
	ledger     *api.Client
	writeCount uint64
}

func main() {
	var listen = flag.String("listen", ":8080", "http listen")
	var ledgerAddr = flag.String("ledger", "http://ledger1:7000", "ledger addr")
	var rangeID = flag.String("range", "demo.accounts:FULL", "range")
	var ownerMap = flag.String("owners", "mysql1=mysql1:3306", "owner map")
	var mysqlUser = flag.String("mysql-user", "restreamx_router", "mysql user")
	var mysqlPass = flag.String("mysql-pass", "router", "mysql pass")
	var mysqlDB = flag.String("mysql-db", "demo", "mysql db")
	var adminUser = flag.String("admin-user", "root", "admin user")
	var adminPass = flag.String("admin-pass", "root", "admin pass")
	var metrics = flag.String("metrics", ":8081", "metrics")
	flag.Parse()

	cfg := config{LedgerAddr: *ledgerAddr, RangeID: *rangeID, OwnerMap: parseOwnerMap(*ownerMap), Timeout: 5 * time.Second, MySQLUser: *mysqlUser, MySQLPass: *mysqlPass, MySQLDB: *mysqlDB, AdminUser: *adminUser, AdminPass: *adminPass}
	r := &router{cfg: cfg, ledger: api.NewClient(*ledgerAddr, 5*time.Second)}

	mux := http.NewServeMux()
	mux.HandleFunc("/write", r.handleWrite)
	mux.HandleFunc("/admin/lease", r.handleLease)
	mux.HandleFunc("/metrics", r.handleMetrics)

	go func() {
		log.Printf("metrics on %s", *metrics)
		_ = http.ListenAndServe(*metrics, http.HandlerFunc(r.handleMetrics))
	}()
	log.Printf("router listening on %s", *listen)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		log.Fatalf("http: %v", err)
	}
}

func parseOwnerMap(raw string) map[string]string {
	out := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

func (r *router) handleLease(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	owner := req.URL.Query().Get("owner")
	if owner == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), r.cfg.Timeout)
	defer cancel()
	lease, err := r.ledger.AcquireLease(ctx, &api.AcquireLeaseRequest{RangeId: r.cfg.RangeID, OwnerId: owner, TtlMs: 30000})
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	if err := r.updateModes(owner); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_ = json.NewEncoder(w).Encode(lease)
}

func (r *router) handleWrite(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var wr writeRequest
	if err := json.NewDecoder(req.Body).Decode(&wr); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), r.cfg.Timeout)
	defer cancel()
	lease, err := r.ledger.GetLease(ctx, r.cfg.RangeID)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("lease not found"))
		return
	}
	host, ok := r.cfg.OwnerMap[lease.OwnerId]
	if !ok {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("owner missing"))
		return
	}
	if err := r.executeTxn(host, &wr); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	payload, _ := json.Marshal(wr)
	seg := &api.Segment{RangeId: r.cfg.RangeID, Epoch: lease.Epoch, TxnId: newTxnID(), PayloadType: "json", PayloadBytes: payload}
	if _, err := r.ledger.AppendSegment(ctx, seg); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	atomic.AddUint64(&r.writeCount, 1)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (r *router) executeTxn(host string, wr *writeRequest) error {
	hostname, port := splitHostPort(host)
	stmt := "START TRANSACTION;"
	switch strings.ToLower(wr.Op) {
	case "insert":
		stmt += fmt.Sprintf("INSERT INTO %s (id, balance, updated_at) VALUES (%d, %v, NOW());", wr.Table, wr.ID, wr.Data["balance"])
	case "update":
		stmt += fmt.Sprintf("UPDATE %s SET balance=%v, updated_at=NOW() WHERE id=%d;", wr.Table, wr.Data["balance"], wr.ID)
	case "delete":
		stmt += fmt.Sprintf("DELETE FROM %s WHERE id=%d;", wr.Table, wr.ID)
	default:
		return fmt.Errorf("unknown op")
	}
	stmt += "COMMIT;"
	return execMySQL(hostname, port, r.cfg.MySQLUser, r.cfg.MySQLPass, r.cfg.MySQLDB, stmt)
}

func (r *router) updateModes(owner string) error {
	for node, host := range r.cfg.OwnerMap {
		hostname, port := splitHostPort(host)
		mode := "REPLICA"
		if node == owner {
			mode = "OWNER"
		}
		stmt := fmt.Sprintf("SET GLOBAL restreamx.mode='%s'; SET GLOBAL restreamx.node_id='%s';", mode, node)
		if err := execMySQL(hostname, port, r.cfg.AdminUser, r.cfg.AdminPass, "mysql", stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *router) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	count := atomic.LoadUint64(&r.writeCount)
	_, _ = fmt.Fprintf(w, "router_write_total %d\n", count)
}

func newTxnID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func splitHostPort(addr string) (string, int) {
	parts := strings.Split(addr, ":")
	if len(parts) == 2 {
		port, _ := strconv.Atoi(parts[1])
		return parts[0], port
	}
	return addr, 3306
}

func execMySQL(host string, port int, user, pass, db, statement string) error {
	args := []string{"-h", host, "-P", strconv.Itoa(port), "-u", user}
	if pass != "" {
		args = append(args, "-p"+pass)
	}
	if db != "" {
		args = append(args, db)
	}
	args = append(args, "-e", statement)
	cmd := exec.Command("mysql", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mysql exec: %s", string(out))
	}
	return nil
}
