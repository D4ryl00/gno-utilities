IMAGE        ?= gnoland:local
GNOKEY_IMAGE ?= gnokey:local
WORK_ROOT    ?= /tmp/gno-val-tests
NODE         ?=

PROJECT_01 = five-validators-reset-four
PROJECT_02 = four-validators-restart-staggered
PROJECT_03 = four-validators-restart-parallel
PROJECT_04 = counter-realm-churn
PROJECT_05 = sentry-ip-rotation

.PHONY: build-gnoland-image build-gnokey-image \
        scenario-01 scenario-02 scenario-03 scenario-04 scenario-05 \
        logs-01 logs-02 logs-03 logs-04 logs-05 \
        clean clean-01 clean-02 clean-03 clean-04 clean-05 \
        status

build-gnoland-image:
	docker build -t $(IMAGE) -f ../gno/Dockerfile --target gnoland ../gno

build-gnokey-image:
	docker build -t $(GNOKEY_IMAGE) -f ../gno/Dockerfile --target gnokey ../gno

clean:
	rm -rf $(WORK_ROOT)

clean-01:
	rm -rf $(WORK_ROOT)/$(PROJECT_01)

clean-02:
	rm -rf $(WORK_ROOT)/$(PROJECT_02)

clean-03:
	rm -rf $(WORK_ROOT)/$(PROJECT_03)

clean-04:
	rm -rf $(WORK_ROOT)/$(PROJECT_04)

clean-05:
	rm -rf $(WORK_ROOT)/$(PROJECT_05)

scenario-01:
	IMAGE_NAME=$(IMAGE) GNOKEY_IMAGE=$(GNOKEY_IMAGE) WORK_ROOT=$(WORK_ROOT) ./scenarios/01_five_validators_reset_four.sh

scenario-02:
	IMAGE_NAME=$(IMAGE) GNOKEY_IMAGE=$(GNOKEY_IMAGE) WORK_ROOT=$(WORK_ROOT) ./scenarios/02_four_validators_restart_staggered.sh

scenario-03:
	IMAGE_NAME=$(IMAGE) GNOKEY_IMAGE=$(GNOKEY_IMAGE) WORK_ROOT=$(WORK_ROOT) ./scenarios/03_four_validators_restart_parallel.sh

scenario-04:
	IMAGE_NAME=$(IMAGE) GNOKEY_IMAGE=$(GNOKEY_IMAGE) WORK_ROOT=$(WORK_ROOT) ./scenarios/04_counter_realm_churn.sh

scenario-05:
	IMAGE_NAME=$(IMAGE) GNOKEY_IMAGE=$(GNOKEY_IMAGE) WORK_ROOT=$(WORK_ROOT) ./scenarios/05_sentry_ip_rotation.sh

logs-01:
	docker compose -p $(PROJECT_01) logs -f $(NODE)

logs-02:
	docker compose -p $(PROJECT_02) logs -f $(NODE)

logs-03:
	docker compose -p $(PROJECT_03) logs -f $(NODE)

logs-04:
	docker compose -p $(PROJECT_04) logs -f $(NODE)

logs-05:
	docker compose -p $(PROJECT_05) logs -f $(NODE)

status:
	@while true; do \
		printf '\033[2J\033[H'; \
		date '+%H:%M:%S'; \
		echo; \
		docker ps --format '{{.Names}}\t{{.Ports}}' | grep -- '->26657/tcp' | sort | \
		while IFS=$$'\t' read -r name ports; do \
			port=$$(printf '%s' "$$ports" | grep -oE '0\.0\.0\.0:[0-9]+' | cut -d: -f2); \
			height=$$(curl -fsS "http://127.0.0.1:$$port/status" 2>/dev/null \
				| jq -r '.result.sync_info.latest_block_height // "offline"' 2>/dev/null \
				|| echo offline); \
			printf '%-40s  height=%s\n' "$$name" "$$height"; \
		done; \
		sleep 2; \
	done

