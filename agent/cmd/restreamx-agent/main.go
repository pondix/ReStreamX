package main

import (
	"context"
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

type payload struct {
	Op    string                 `json:"op"`
	Table string                 `json:"table"`
	ID    int                    `json:"id"`
	Data  map[string]interface{} `json:"data"`
}

type agent struct {
	ledger    *api.Client
	host      string
	port      int
	user      string
	pass      string
	db        string
	applied   uint64
	lastEpoch uint64
}

func main() {
	var ledgerAddr = flag.String("ledger", "http://ledger1:7000", "ledger addr")
	var mysqlHost = flag.String("mysql-host", "mysql1", "mysql host")
	var mysqlPort = flag.Int("mysql-port", 3306, "mysql port")
	var mysqlUser = flag.String("mysql-user", "restreamx_apply", "mysql user")
	var mysqlPass = flag.String("mysql-pass", "apply", "mysql pass")
	var mysqlDB = flag.String("mysql-db", "demo", "mysql db")
	var metrics = flag.String("metrics", ":9090", "metrics")
	flag.Parse()

	ag := &agent{ledger: api.NewClient(*ledgerAddr, 5*time.Second), host: *mysqlHost, port: *mysqlPort, user: *mysqlUser, pass: *mysqlPass, db: *mysqlDB}
	go ag.subscribeLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", ag.handleMetrics)
	log.Printf("metrics on %s", *metrics)
	if err := http.ListenAndServe(*metrics, mux); err != nil {
		log.Fatalf("metrics: %v", err)
	}
}

func (a *agent) subscribeLoop() {
	var from uint64 = 1
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		segs, err := a.ledger.Subscribe(ctx, from)
		cancel()
		if err != nil {
			log.Printf("subscribe: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		for _, seg := range segs {
			if seg.Epoch < atomic.LoadUint64(&a.lastEpoch) {
				continue
			}
			if err := a.applySegment(seg); err != nil {
				log.Printf("apply error: %v", err)
				continue
			}
			atomic.StoreUint64(&a.applied, seg.CommitIndex)
			atomic.StoreUint64(&a.lastEpoch, seg.Epoch)
			from = seg.CommitIndex + 1
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (a *agent) applySegment(seg *api.Segment) error {
	var p payload
	if err := json.Unmarshal(seg.PayloadBytes, &p); err != nil {
		return err
	}
	stmt := "START TRANSACTION;"
	switch p.Op {
	case "insert":
		stmt += fmt.Sprintf("INSERT INTO %s (id, balance, updated_at) VALUES (%d, %v, NOW()) ON DUPLICATE KEY UPDATE balance=VALUES(balance), updated_at=VALUES(updated_at);", p.Table, p.ID, p.Data["balance"])
	case "update":
		stmt += fmt.Sprintf("UPDATE %s SET balance=%v, updated_at=NOW() WHERE id=%d;", p.Table, p.Data["balance"], p.ID)
	case "delete":
		stmt += fmt.Sprintf("DELETE FROM %s WHERE id=%d;", p.Table, p.ID)
	default:
		return nil
	}
	stmt += fmt.Sprintf("INSERT INTO rlr_meta.applied_segments (range_id, epoch, txn_id, commit_index, applied_at) VALUES ('%s', %d, '%s', %d, NOW()) ON DUPLICATE KEY UPDATE commit_index=VALUES(commit_index);", seg.RangeId, seg.Epoch, seg.TxnId, seg.CommitIndex)
	stmt += "COMMIT;"
	return execMySQL(a.host, a.port, a.user, a.pass, a.db, stmt)
}

func (a *agent) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	applied := atomic.LoadUint64(&a.applied)
	lastEpoch := atomic.LoadUint64(&a.lastEpoch)
	_, _ = fmt.Fprintf(w, "agent_applied_index %d\n", applied)
	_, _ = fmt.Fprintf(w, "agent_last_epoch %d\n", lastEpoch)
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
		return fmt.Errorf("mysql exec: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
