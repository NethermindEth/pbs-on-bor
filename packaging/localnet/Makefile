
.PHONY: bins clear-term wipe wipe-bins wipe-logs start1 start2

LOGTS := ${shell date +'%a-%H'}

wipe:
	rm -rf data/node_?

wipe-logs:
	rm -f logs/node*.log

clear-term:
	@clear

start1: clear-term
	bin/bor server --config config/node1_config.toml 2>&1 | tee -a "logs/node1-${LOGTS}.log"

start2: clear-term
	bin/builder server --config config/node2_config.toml 2>&1 | tee -a "logs/node2-${LOGTS}.log"

wipe-bins:
	rm -f bin/*

TARGETDIR   := build/bin
BORDIR     := ../bor-proposer
BUILDERDIR := ../bor-builder
bins:
	cd ${BORDIR} && make bor && cp ./${TARGETDIR}/bor ../localnet/bin
	cd ${BUILDERDIR} && make builder && cp ./${TARGETDIR}/builder ../localnet/bin
	ls -lA ./bin
