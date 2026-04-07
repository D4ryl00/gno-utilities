#!/usr/bin/env zsh
set -euo pipefail

ROOT_DIR="${0:A:h:h}"
source "${ROOT_DIR}/lib/scenario.sh"

scenario_init "four-validators-restart-staggered"
trap scenario_finish EXIT

gen_validator val1
gen_validator val2
gen_validator val3
gen_validator val4

prepare_network
start_all_nodes
wait_for_seconds 60

stop_validator val1
stop_validator val2
stop_validator val3
stop_validator val4
wait_for_seconds 5

start_validator val1
wait_for_seconds 10
start_validator val2
wait_for_seconds 10
start_validator val3
wait_for_seconds 10
start_validator val4
wait_for_seconds 30

print_cluster_status
