#!/usr/bin/env bash
set -euo pipefail

go build ./ledger/cmd/restreamx-ledgerd

go build ./router/cmd/restreamx-router

go build ./agent/cmd/restreamx-agent
