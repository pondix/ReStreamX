.PHONY: build package up down e2e

build:
	go build ./ledger/cmd/restreamx-ledgerd
	go build ./router/cmd/restreamx-router
	go build ./agent/cmd/restreamx-agent

up:
	deploy/scripts/up.sh

down:
	deploy/scripts/down.sh

e2e:
	deploy/scripts/e2e.sh

package: build
	mkdir -p packages/output
	nfpm pkg -f packages/nfpm/ledger.yaml -p packages/output/restreamx-ledgerd.deb
	nfpm pkg -f packages/nfpm/router.yaml -p packages/output/restreamx-router.deb
	nfpm pkg -f packages/nfpm/agent.yaml -p packages/output/restreamx-agent.deb
	cmake -S mysql-plugin -B build/mysql-plugin
	cmake --build build/mysql-plugin
	cp build/mysql-plugin/restreamx.so ./restreamx.so
	nfpm pkg -f packages/nfpm/plugin.yaml -p packages/output/restreamx-plugin.deb
