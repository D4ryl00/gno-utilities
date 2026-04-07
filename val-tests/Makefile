IMAGE ?= gno-scenario-all:local

.PHONY: build-image clean scenario-01 scenario-02 scenario-03 scenario-04 scenario-05

build-image:
	docker build -t $(IMAGE) -f ../gno/Dockerfile --target all ../gno

clean:
	rm -rf .work

scenario-01:
	./scenarios/01_five_validators_reset_four.sh

scenario-02:
	./scenarios/02_four_validators_restart_staggered.sh

scenario-03:
	./scenarios/03_four_validators_restart_parallel.sh

scenario-04:
	./scenarios/04_counter_realm_churn.sh

scenario-05:
	./scenarios/05_sentry_ip_rotation.sh

