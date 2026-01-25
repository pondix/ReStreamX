#!/usr/bin/env bash
set -euo pipefail

docker compose -f deploy/docker/docker-compose.yml up --build -d
