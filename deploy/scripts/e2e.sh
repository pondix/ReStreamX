#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -f deploy/docker/docker-compose.yml)
router=${ROUTER_ADDR:-http://localhost:8080}

wait_mysql() {
  local svc=$1
  for _ in $(seq 1 60); do
    if "${compose[@]}" exec -T "$svc" mysqladmin -uroot -proot ping --silent >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "mysql $svc not ready" >&2
  return 1
}

mysql_count() {
  local svc=$1
  local table=$2
  "${compose[@]}" exec -T "$svc" mysql -uroot -proot -N -e "SELECT COUNT(*) FROM demo.${table};"
}

mysql_checksum() {
  local svc=$1
  local table=$2
  "${compose[@]}" exec -T "$svc" mysql -uroot -proot -N -e "SELECT IFNULL(BIT_XOR(CRC32(CONCAT(id,':',balance))),0) FROM demo.${table};"
}

write_ops() {
  local start=$1
  local end=$2
  for i in $(seq "$start" "$end"); do
    payload=$(printf '{"op":"insert","table":"accounts","id":%d,"data":{"balance":%d}}' "$i" "$i")
    curl -s -X POST "${router}/write" -d "$payload" >/dev/null
    payload=$(printf '{"op":"insert","table":"orders","id":%d,"data":{"balance":%d}}' "$i" "$i")
    curl -s -X POST "${router}/write" -d "$payload" >/dev/null
  done
}

wait_mysql mysql1
wait_mysql mysql2
wait_mysql mysql3

curl -s -X POST "${router}/admin/lease?owner=mysql1" >/dev/null

write_ops 1 20000

sleep 5

count1=$(mysql_count mysql1 accounts)
count2=$(mysql_count mysql2 accounts)
count3=$(mysql_count mysql3 accounts)
orders1=$(mysql_count mysql1 orders)
orders2=$(mysql_count mysql2 orders)
orders3=$(mysql_count mysql3 orders)
if [[ "$count1" -ne 20000 || "$count2" -ne 20000 || "$count3" -ne 20000 || "$orders1" -ne 20000 || "$orders2" -ne 20000 || "$orders3" -ne 20000 ]]; then
  echo "FAIL: counts mismatch after initial load" >&2
  exit 1
fi
chk1=$(mysql_checksum mysql1 accounts)
chk2=$(mysql_checksum mysql2 accounts)
chk3=$(mysql_checksum mysql3 accounts)
if [[ "$chk1" != "$chk2" || "$chk2" != "$chk3" ]]; then
  echo "FAIL: checksum mismatch after initial load" >&2
  exit 1
fi

"${compose[@]}" stop mysql2 agent2
write_ops 20001 21000
"${compose[@]}" start mysql2 agent2
wait_mysql mysql2

for _ in $(seq 1 30); do
  count2=$(mysql_count mysql2 accounts)
  orders2=$(mysql_count mysql2 orders)
  if [[ "$count2" -ge 21000 && "$orders2" -ge 21000 ]]; then
    break
  fi
  sleep 1
 done
if [[ "$count2" -ne 21000 || "$orders2" -ne 21000 ]]; then
  echo "FAIL: mysql2 did not catch up" >&2
  exit 1
fi
chk1=$(mysql_checksum mysql1 orders)
chk2=$(mysql_checksum mysql2 orders)
chk3=$(mysql_checksum mysql3 orders)
if [[ "$chk1" != "$chk2" || "$chk2" != "$chk3" ]]; then
  echo "FAIL: checksum mismatch after catch-up" >&2
  exit 1
fi

curl -s -X POST "${router}/admin/lease?owner=mysql3" >/dev/null
write_ops 21001 21500

set +e
"${compose[@]}" exec -T mysql1 mysql -uroot -proot -e "INSERT INTO demo.accounts (id, balance, updated_at) VALUES (999999, 1, NOW());" >/dev/null 2>&1
write_rc=$?
set -e
if [[ $write_rc -eq 0 ]]; then
  echo "FAIL: mysql1 accepted write after failover" >&2
  exit 1
fi

count1=$(mysql_count mysql1 accounts)
count2=$(mysql_count mysql2 accounts)
count3=$(mysql_count mysql3 accounts)
orders1=$(mysql_count mysql1 orders)
orders2=$(mysql_count mysql2 orders)
orders3=$(mysql_count mysql3 orders)
if [[ "$count1" -ne 21500 || "$count2" -ne 21500 || "$count3" -ne 21500 || "$orders1" -ne 21500 || "$orders2" -ne 21500 || "$orders3" -ne 21500 ]]; then
  echo "FAIL: counts mismatch after failover" >&2
  exit 1
fi
chk1=$(mysql_checksum mysql1 accounts)
chk2=$(mysql_checksum mysql2 accounts)
chk3=$(mysql_checksum mysql3 accounts)
if [[ "$chk1" != "$chk2" || "$chk2" != "$chk3" ]]; then
  echo "FAIL: checksum mismatch after failover" >&2
  exit 1
fi

echo "PASS"
