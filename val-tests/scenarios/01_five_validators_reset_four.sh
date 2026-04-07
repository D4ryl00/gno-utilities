#!/usr/bin/env zsh
set -euo pipefail

ROOT_DIR="${0:A:h:h}"
source "${ROOT_DIR}/lib/scenario.sh"

scenario_init "five-validators-reset-four"
trap scenario_finish EXIT

gen_validator val1
gen_validator val2
gen_validator val3
gen_validator val4
gen_validator val5

prepare_network
start_all_nodes
wait_for_seconds 60

stop_validator val2
stop_validator val3
stop_validator val4
stop_validator val5

reset_validator val2
reset_validator val3
reset_validator val4
reset_validator val5

start_validator val2
start_validator val3
start_validator val4
start_validator val5
wait_for_seconds 60

print_cluster_status
