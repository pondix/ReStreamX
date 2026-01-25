#!/usr/bin/env bash
set -euo pipefail

docker compose -f deploy/docker/docker-compose.yml logs -f --tail=200
