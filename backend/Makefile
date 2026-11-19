SHELL := /bin/sh

ROOT_DIR := $(abspath ..)
ARTIFACTS_DIR ?= $(CURDIR)
TEST_IMAGE ?= chimera-backend-test:local
GOBCO_VERSION ?= v1.3.4
ALLURE_IMAGE ?= tobix/allure-cli:2
ALLURE_PORT ?= 5252
NGINX_IMAGE ?= nginx:1.27-alpine
BUF_IMAGE ?= bufbuild/buf:1.59.0
BUF_AGAINST ?= .git\#branch=develop,subdir=backend/contracts
NETSHOOT_IMAGE ?= nicolaka/netshoot:v0.13
E2E_OUT ?= $(ARTIFACTS_DIR)/e2e-out

# Every module of go.work. Patterns name modules because backend/ itself is
# not a module: go test runs in workspace mode across all of them.
TEST_PACKAGES ?= chimera/platform/... chimera/testkit/... chimera/identity/... chimera/catalog/... chimera/social/... chimera/gateway/...
# INTEGRATION_SERVICES — each one gets its own empty database per run.
INTEGRATION_SERVICES ?= identity catalog social
# INTEGRATION_TAGS — build tag файлов с //go:build integration.
# INTEGRATION_RUN — регулярка go test -run.XXxTest
INTEGRATION_TAGS ?= integration
INTEGRATION_RUN ?= Integration
POSTGRES_TEST_IMAGE ?= postgres:17-alpine
COVERAGE_PACKAGES ?= chimera/identity/internal/... chimera/catalog/internal/... chimera/social/internal/... chimera/gateway/internal/...
GOBCO_PACKAGES ?= ./identity/internal/domain ./identity/internal/usecase ./identity/internal/infra/postgres \
	./catalog/internal/domain ./catalog/internal/usecase ./catalog/internal/infra/postgres \
	./social/internal/domain ./social/internal/usecase ./social/internal/infra/postgres
# GO_TEST_P — сколько пакетов go test запускает сразу (-p).
# GO_TEST_PARALLEL — сколько тестов с t.Parallel() идёт внутри пакета (-parallel).
GO_TEST_P ?= 1
GO_TEST_PARALLEL ?= 4
SHUFFLE_SEED ?= on
TEST_UID := $(shell id -u)
TEST_GID := $(shell id -g)

RUN_TEST_IMAGE = docker run --rm --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp -e ALLURE_RESULTS_DIR=/artifacts/allure-results -v "$(ARTIFACTS_DIR):/artifacts" "$(TEST_IMAGE)"
GO_TEST_FLAGS = -count=1 -p=$(GO_TEST_P) -parallel=$(GO_TEST_PARALLEL)

.PHONY: help proto proto-lint proto-breaking test-image test test-shuffle test-offline stand-up stand-gc stand-down test-integration test-e2e demo-capture demo-requests e2e-dev app-logs test-logs test-all coverage gobco-condition gobco-branch allure-results allure-meta allure-render allure-report allure-open spec-matrix spec-check clean-test-results \
	k8s-keys k8s-secrets k8s-images k8s-up k8s-down k8s-status k8s-logs k8s-e2e \
	k8s-monitoring k8s-chaos k8s-load k8s-replay-dlq perf-run perf-prepare perf-restore
.NOTPARALLEL: test-all

help:
	@printf '%s\n' \
		'proto            Regenerate Go code from contracts/proto with buf' \
		'proto-lint       Lint contracts/proto with buf' \
		'proto-breaking   Fail on breaking proto changes against develop' \
		'test-image       Build the reusable Docker test image' \
		'test             Run the standard test suite' \
		'test-shuffle     Run tests with a recorded/random shuffle seed' \
		'test-offline     Run the prebuilt image without network access' \
		'stand-up         Start the shared Postgres and RustFS stand' \
		'stand-gc         Drop t_* databases that have no sessions' \
		'stand-down       Stop the stand without deleting its volumes' \
		'test-integration Run integration tests of every service against the running stand' \
		'test-e2e         Run the demo against a private compose stack (Gateway, services, Kafka)' \
		'demo-capture     Replay the demo with curl and write pcap captures' \
		'demo-requests    Run the curl demo once against the running dev Gateway (RUN=name)' \
		'e2e-dev          Run the Go demo once against the running dev Gateway (RUN=name)' \
		'app-logs         Follow dev service logs, only one run with RUN=curl-1 or RUN=go-1' \
		'test-logs        Follow integration run logs, one run with RUN=1' \
		'test-all         Run unit suites, coverage, gobco, and integration tests' \
		'coverage         Write statement profile and HTML coverage' \
		'gobco-condition  Write gobco condition coverage output' \
		'gobco-branch     Write gobco branch coverage output' \
		'allure-results   Run unit, integration, and e2e tests into allure-results' \
		'allure-render    Render allure-report from existing results (no test run)' \
		'allure-report    Run allure-results, then allure-render' \
		'spec-matrix      Rewrite the TEST_SPEC.md matrix from allure-results' \
		'spec-check       Fail when the TEST_SPEC.md matrix differs from allure-results' \
		'allure-open      Serve the report at http://127.0.0.1:$(ALLURE_PORT)' \
		'k8s-keys         Generate the Ed25519 JWT key pair for the minikube overlay (once)' \
		'k8s-secrets      Write random passwords to the overlay secrets.env (once)' \
		'k8s-images       Build all images straight into the minikube node' \
		'k8s-up           Enable ingress, apply the overlay, wait for every workload' \
		'k8s-down         Delete the chimera namespace with its volumes' \
		'k8s-status       Show pods, services, ingresses and jobs' \
		'k8s-logs         Follow one workload, SVC=gateway|identity|catalog|social|...' \
		'k8s-e2e          Run the Go demo against the minikube ingress' \
		'k8s-monitoring   Print the Grafana, Prometheus and Tempo addresses' \
		'k8s-chaos        Take catalog pods and the Kafka broker away under k6 load' \
		'k8s-load         Run loadtest/balance.js against the minikube edge' \
		'k8s-replay-dlq   Move DLQ records back to their topics' \
		'perf-run         Perf scenario. PROFILE=quick|full SCENARIO=ramp|stress|soak|trio|all FAULT_RATE=0|0.1|0.2'

BUF = docker run --rm --user $(TEST_UID):$(TEST_GID) -e BUF_CACHE_DIR=/tmp/buf -v "$(ROOT_DIR):/repo" -w /repo/backend/contracts "$(BUF_IMAGE)"

proto:
	$(BUF) generate

proto-lint:
	$(BUF) lint

proto-breaking:
	$(BUF) breaking --against "/repo/$(BUF_AGAINST)"

test-image:
	docker build -f Dockerfile.test --build-arg GOBCO_VERSION="$(GOBCO_VERSION)" --tag "$(TEST_IMAGE)" .

$(ARTIFACTS_DIR):
	mkdir -p "$(ARTIFACTS_DIR)"

test: test-image | $(ARTIFACTS_DIR)
	$(RUN_TEST_IMAGE) go test $(GO_TEST_FLAGS) $(TEST_PACKAGES)

test-shuffle: test-image | $(ARTIFACTS_DIR)
	$(RUN_TEST_IMAGE) go test -v $(GO_TEST_FLAGS) -shuffle="$(SHUFFLE_SEED)" $(TEST_PACKAGES)

test-offline: | $(ARTIFACTS_DIR)
	@docker image inspect "$(TEST_IMAGE)" >/dev/null 2>&1 || { printf '%s\n' 'test image is missing; run make test-image while online first' >&2; exit 1; }
	docker run --rm --pull=never --network none --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp -e ALLURE_RESULTS_DIR=/artifacts/allure-results -v "$(ARTIFACTS_DIR):/artifacts" "$(TEST_IMAGE)" go test $(GO_TEST_FLAGS) $(TEST_PACKAGES)

stand-up:
	sh scripts/stand-up.sh

stand-gc:
	sh scripts/stand-gc.sh

stand-down:
	sh scripts/stand-down.sh

test-integration: | $(ARTIFACTS_DIR)
	@test -f "$(ROOT_DIR)/.env" || { printf '%s\n' 'missing ../.env; copy .env.example to .env' >&2; exit 1; }
	STAND_ALLURE_SUITE=integration sh scripts/stand-require.sh
	$(MAKE) test-image
	ARTIFACTS_DIR="$(ARTIFACTS_DIR)" TEST_IMAGE="$(TEST_IMAGE)" TEST_UID="$(TEST_UID)" TEST_GID="$(TEST_GID)" \
		INTEGRATION_TAGS="$(INTEGRATION_TAGS)" INTEGRATION_RUN="$(INTEGRATION_RUN)" RUN="$(RUN)" \
		sh scripts/test-integration.sh $(INTEGRATION_SERVICES)

test-e2e: | $(ARTIFACTS_DIR)
	@test -f "$(ROOT_DIR)/.env" || { printf '%s\n' 'missing ../.env; copy .env.example to .env' >&2; exit 1; }
	$(MAKE) test-image
	ARTIFACTS_DIR="$(ARTIFACTS_DIR)" TEST_IMAGE="$(TEST_IMAGE)" \
		TEST_UID="$(TEST_UID)" TEST_GID="$(TEST_GID)" RUN="$(RUN)" \
		sh scripts/stand-e2e.sh

demo-capture:
	NETSHOOT_IMAGE="$(NETSHOOT_IMAGE)" sh scripts/demo-capture.sh

DEV_API_URL ?= http://127.0.0.1:8080
DEV_SERVICES ?= gateway identity catalog social
RUN ?=

demo-requests:
	@run="$(RUN)"; [ -n "$$run" ] || run=$$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n'); \
	mkdir -p "$(ARTIFACTS_DIR)/e2e-out/runs"; \
	printf '%s\n' "curl run: make app-logs RUN=curl-$$run"; \
	docker run --rm --network host --user $(TEST_UID):$(TEST_GID) \
		-e E2E_API_URL="$(DEV_API_URL)" -e E2E_OUT=/out -e E2E_RUN="$$run" \
		-v "$(ARTIFACTS_DIR)/e2e-out/runs:/out" \
		-v "$(CURDIR)/scripts/demo-requests.sh:/demo-requests.sh:ro" \
		"$(NETSHOOT_IMAGE)" sh /demo-requests.sh

e2e-dev: test-image | $(ARTIFACTS_DIR)
	@run="$(RUN)"; [ -n "$$run" ] || run=$$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n'); \
	printf '%s\n' "go run: make app-logs RUN=go-$$run"; \
	docker run --rm --network host --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp \
		-e ALLURE_RESULTS_DIR=/artifacts/allure-results \
		-e E2E_API_URL="$(DEV_API_URL)" -e E2E_RUN="$$run" \
		-v "$(ARTIFACTS_DIR):/artifacts" "$(TEST_IMAGE)" \
		go test -count=1 -tags=e2e chimera/e2e/...

app-logs:
	@pattern='run_id=$(RUN)( |$$)'; [ -n "$(RUN)" ] || pattern=''; \
	docker compose --project-directory "$(ROOT_DIR)" logs -f --no-log-prefix --since 10m $(DEV_SERVICES) 2>&1 | grep --line-buffered -E -- "$$pattern"

test-logs:
	@tail -n +1 -f "$(ARTIFACTS_DIR)"/test-logs/it-$(or $(RUN),*).log

coverage: test-image | $(ARTIFACTS_DIR)
	$(RUN_TEST_IMAGE) sh -ec 'go test $(GO_TEST_FLAGS) -covermode=atomic -coverprofile=/artifacts/coverage.out $(COVERAGE_PACKAGES); go tool cover -func=/artifacts/coverage.out; go tool cover -html=/artifacts/coverage.out -o=/artifacts/coverage.html'

gobco-condition: test-image | $(ARTIFACTS_DIR)
	$(RUN_TEST_IMAGE) sh -ec ': > /artifacts/condition-coverage.txt; for pkg in $(GOBCO_PACKAGES); do printf "\n== %s ==\n" "$$pkg" >> /artifacts/condition-coverage.txt; gobco -list-all -test=-p=$(GO_TEST_P) -test=-parallel=$(GO_TEST_PARALLEL) "$$pkg" >> /artifacts/condition-coverage.txt; done; cat /artifacts/condition-coverage.txt'

gobco-branch: test-image | $(ARTIFACTS_DIR)
	$(RUN_TEST_IMAGE) sh -ec ': > /artifacts/branch-coverage.txt; for pkg in $(GOBCO_PACKAGES); do printf "\n== %s ==\n" "$$pkg" >> /artifacts/branch-coverage.txt; gobco -branch -list-all -test=-p=$(GO_TEST_P) -test=-parallel=$(GO_TEST_PARALLEL) "$$pkg" >> /artifacts/branch-coverage.txt; done; cat /artifacts/branch-coverage.txt'

allure-results: test-image | $(ARTIFACTS_DIR)
	rm -rf "$(ARTIFACTS_DIR)/allure-results"
	mkdir -p "$(ARTIFACTS_DIR)/allure-results"
	docker run --rm --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp \
		-e ALLURE_RESULTS_DIR=/artifacts/allure-results \
		-v "$(ARTIFACTS_DIR):/artifacts" "$(TEST_IMAGE)" \
		go test $(GO_TEST_FLAGS) $(TEST_PACKAGES)
	$(MAKE) test-integration
	$(MAKE) test-e2e

allure-meta: | $(ARTIFACTS_DIR)
	mkdir -p "$(ARTIFACTS_DIR)/allure-results"
	cp allure/categories.json "$(ARTIFACTS_DIR)/allure-results/categories.json"
	@{ \
		printf 'Go=%s\n' "$$(docker run --rm "$(TEST_IMAGE)" go version 2>/dev/null | cut -d' ' -f3)"; \
		printf 'PostgreSQL.image=%s\n' "$(POSTGRES_TEST_IMAGE)"; \
		printf 'Commit=%s\n' "$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"; \
		printf 'Branch=%s\n' "$$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"; \
		if [ -n "$${GITHUB_RUN_ID:-}" ]; then printf 'Run=GitHub Actions run %s\n' "$$GITHUB_RUN_ID"; else printf 'Run=local\n'; fi; \
		printf 'Suites=unit (domain, usecase, postgres stub and pgxmock, gateway), integration (one empty PostgreSQL database per service and run), e2e (compose stack through the Gateway)\n'; \
		printf 'Specification=backend/TEST_SPEC.md\n'; \
	} > "$(ARTIFACTS_DIR)/allure-results/environment.properties"

allure-render: allure-meta
	rm -rf "$(ARTIFACTS_DIR)/allure-report"
	mkdir -p "$(ARTIFACTS_DIR)/allure-report"
	docker run --rm --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp \
		-v "$(ARTIFACTS_DIR):/work" "$(ALLURE_IMAGE)" \
		generate /work/allure-results --clean -o /work/allure-report

allure-report: allure-results
	$(MAKE) allure-render

spec-matrix:
	sh scripts/spec-matrix.sh "$(ARTIFACTS_DIR)/allure-results" --write TEST_SPEC.md

spec-check:
	sh scripts/spec-matrix.sh "$(ARTIFACTS_DIR)/allure-results" --check TEST_SPEC.md

allure-open:
	@ids=$$(docker ps -q --filter "publish=$(ALLURE_PORT)"); \
	if ls "$(ARTIFACTS_DIR)/allure-results/"*-result.json >/dev/null 2>&1; then \
		$(MAKE) allure-render; \
		if [ -n "$$ids" ]; then docker stop $$ids >/dev/null; fi; \
	elif [ -n "$$ids" ]; then \
		printf '%s\n' "Allure report is already served at http://127.0.0.1:$(ALLURE_PORT)"; \
		printf '%s\n' "This page was rendered earlier and does not include the latest make test-e2e." >&2; \
		exit 0; \
	fi; \
	test -f "$(ARTIFACTS_DIR)/allure-report/index.html" || { printf '%s\n' 'report is missing; run make allure-report first' >&2; exit 1; }; \
	docker image inspect "$(NGINX_IMAGE)" >/dev/null 2>&1 || docker pull "$(NGINX_IMAGE)"; \
	printf '%s\n' "serving Allure report at http://127.0.0.1:$(ALLURE_PORT)"; \
	docker run --rm --pull=never --name chimera-allure -p "$(ALLURE_PORT):80" \
		-v "$(ARTIFACTS_DIR)/allure-report:/usr/share/nginx/html:ro" \
		"$(NGINX_IMAGE)"

test-all: test test-shuffle test-offline coverage gobco-condition gobco-branch test-integration

clean-test-results:
	rm -rf "$(ARTIFACTS_DIR)/allure-results" "$(ARTIFACTS_DIR)/allure-report" "$(ARTIFACTS_DIR)/e2e-out" "$(ARTIFACTS_DIR)/test-logs"
	rm -f "$(ARTIFACTS_DIR)/allure-report.html"
	rm -f "$(ARTIFACTS_DIR)/coverage.out" "$(ARTIFACTS_DIR)/coverage.html"
	rm -f "$(ARTIFACTS_DIR)/condition-coverage.txt" "$(ARTIFACTS_DIR)/branch-coverage.txt"

# --- minikube -------------------------------------------------------------

KUBECTL ?= minikube kubectl --
K8S_OVERLAY ?= $(ROOT_DIR)/deploy/k8s/overlays/minikube
K8S_NAMESPACE ?= chimera
K8S_HOST ?= chimera.192.168.49.2.nip.io
K8S_SERVICES := identity catalog social gateway
K8S_WORKLOADS := $(K8S_SERVICES) edge web mirror adminer otel-collector tempo loki prometheus grafana kafka-exporter
K6_IMAGE ?= grafana/k6:0.57.0
K8S_IMAGE_TAG ?= dev
SVC ?= gateway

k8s-keys:
	@if [ -s "$(K8S_OVERLAY)/keys/jwt.pem" ]; then echo "keys exist: $(K8S_OVERLAY)/keys"; exit 0; fi; \
	mkdir -p "$(K8S_OVERLAY)/keys" && \
	docker build -q -t chimera-keygen:local "$(ROOT_DIR)/deploy/keygen" >/dev/null && \
	docker run --rm --user $(TEST_UID):$(TEST_GID) -e KEY_OWNER_UID=$(TEST_UID) \
		-v "$(K8S_OVERLAY)/keys:/keys/private" -v "$(K8S_OVERLAY)/keys:/keys/public" chimera-keygen:local && \
	echo "wrote $(K8S_OVERLAY)/keys/jwt.pem and jwt.pub"

K8S_SECRET_KEYS := POSTGRES_PASSWORD POSTGRES_REPLICATION_PASSWORD \
	IDENTITY_DB_PASSWORD CATALOG_DB_PASSWORD SOCIAL_DB_PASSWORD \
	IDENTITY_DB_READ_PASSWORD CATALOG_DB_READ_PASSWORD SOCIAL_DB_READ_PASSWORD \
	S3_SECRET_KEY GRAFANA_ADMIN_PASSWORD ADMIN_PASSWORD

# Adds only the keys secrets.env lacks, so existing passwords keep matching the volumes.
k8s-secrets:
	@umask 077; file="$(K8S_OVERLAY)/secrets.env"; touch "$$file"; \
	for key in $(K8S_SECRET_KEYS); do \
		grep -q "^$$key=" "$$file" || printf '%s=%s\n' "$$key" "$$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')" >> "$$file"; \
	done; \
	grep -q '^S3_ACCESS_KEY=' "$$file" || printf 'S3_ACCESS_KEY=chimera\n' >> "$$file"; \
	echo "secrets in $$file"

k8s-images:
	@for svc in $(K8S_SERVICES); do \
		echo "building chimera/$$svc:$(K8S_IMAGE_TAG)"; \
		minikube image build -t "chimera/$$svc:$(K8S_IMAGE_TAG)" -f "$$svc/Dockerfile" . || exit 1; \
	done
	minikube image build -t "chimera/web:$(K8S_IMAGE_TAG)" "$(ROOT_DIR)/frontend"
	minikube image build -t "chimera/edge:$(K8S_IMAGE_TAG)" -f edge/Dockerfile "$(ROOT_DIR)"
	minikube image build -t "chimera/mirror:$(K8S_IMAGE_TAG)" -f edge/mirror.Dockerfile "$(ROOT_DIR)"

k8s-up: k8s-keys k8s-secrets
	minikube addons enable ingress
	$(KUBECTL) -n ingress-nginx wait --for=condition=Available deployment/ingress-nginx-controller --timeout=180s
	@# The ingress admission webhook starts answering a little after the controller is Available.
	@i=0; until $(KUBECTL) apply -k "$(K8S_OVERLAY)"; do \
		i=$$((i + 1)); [ "$$i" -lt 12 ] || exit 1; sleep 5; \
	done
	@for s in postgres postgres-replica rustfs kafka; do \
		$(KUBECTL) -n $(K8S_NAMESPACE) rollout status "statefulset/$$s" --timeout=600s || exit 1; \
	done
	$(KUBECTL) -n $(K8S_NAMESPACE) wait --for=condition=Complete job/kafka-topics --timeout=300s
	@for d in $(K8S_WORKLOADS); do \
		$(KUBECTL) -n $(K8S_NAMESPACE) rollout status "deployment/$$d" --timeout=300s || exit 1; \
	done
	$(KUBECTL) -n $(K8S_NAMESPACE) rollout status daemonset/promtail --timeout=300s
	@echo "edge http://$(K8S_HOST)/  api http://$(K8S_HOST)/api/v1/tracks  docs http://$(K8S_HOST)/api/v1/docs/"
	@echo "legacy http://$(K8S_HOST)/legacy/  mirror http://$(K8S_HOST)/mirror/  monitoring http://$(K8S_HOST)/monitoring/"

k8s-down:
	$(KUBECTL) delete namespace $(K8S_NAMESPACE) --ignore-not-found --wait=true

k8s-status:
	$(KUBECTL) -n $(K8S_NAMESPACE) get pods,svc,ingress,jobs,pvc -o wide

k8s-logs:
	$(KUBECTL) -n $(K8S_NAMESPACE) logs -f --tail=200 "$$(case $(SVC) in postgres|rustfs|kafka) echo statefulset;; kafka-topics) echo job;; *) echo deployment;; esac)/$(SVC)"

k8s-e2e: test-image | $(ARTIFACTS_DIR)
	@run="$(RUN)"; [ -n "$$run" ] || run=$$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n'); \
	printf '%s\n' "go run against http://$(K8S_HOST): make k8s-logs SVC=gateway | grep go-$$run"; \
	docker run --rm --network host --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp \
		-e ALLURE_RESULTS_DIR=/artifacts/allure-results \
		-e E2E_API_URL="http://$(K8S_HOST)/api" -e E2E_RUN="$$run" \
		-v "$(ARTIFACTS_DIR):/artifacts" "$(TEST_IMAGE)" \
		go test -count=1 -tags=e2e chimera/e2e/...

k8s-monitoring:
	@echo "grafana    http://$(K8S_HOST)/monitoring/ (anonymous viewer; admin password: GRAFANA_ADMIN_PASSWORD in secrets.env)"
	@echo "prometheus $(KUBECTL) -n $(K8S_NAMESPACE) port-forward svc/prometheus 9090"
	@echo "tempo      $(KUBECTL) -n $(K8S_NAMESPACE) port-forward svc/tempo 3200"

# Drops one Catalog pod, then the whole Catalog, then the Postgres replicas, while k6
# reads the feed. docs/resilience.md has the phases and the measured numbers.
k8s-chaos: | $(ARTIFACTS_DIR)
	KUBECTL="$(KUBECTL)" K8S_NAMESPACE=$(K8S_NAMESPACE) K8S_HOST=$(K8S_HOST) K6_IMAGE=$(K6_IMAGE) \
		ARTIFACTS_DIR="$(ARTIFACTS_DIR)" sh "$(ROOT_DIR)/deploy/scripts/chaos.sh"

k8s-load: | $(ARTIFACTS_DIR)
	docker run --rm --network host --user $(TEST_UID):$(TEST_GID) \
		-e BASE_URL="http://$(K8S_HOST)" -e SUMMARY_DIR=/out \
		-e GATEWAY_PODS="$$( $(KUBECTL) -n $(K8S_NAMESPACE) get pods -l app.kubernetes.io/name=gateway -o jsonpath='{.items[*].metadata.name}' )" \
		-e CATALOG_PODS="$$( $(KUBECTL) -n $(K8S_NAMESPACE) get pods -l app.kubernetes.io/name=catalog -o jsonpath='{.items[*].metadata.name}' )" \
		-v "$(ROOT_DIR)/loadtest:/loadtest:ro" -v "$(ARTIFACTS_DIR):/out" \
		"$(K6_IMAGE)" run /loadtest/balance.js

k8s-replay-dlq:
	KUBECTL="$(KUBECTL)" K8S_NAMESPACE=$(K8S_NAMESPACE) sh "$(ROOT_DIR)/deploy/kafka/replay-dlq.sh"

# CPU pinning is optional: ISOLATE_CPUS=1; sudo additionally configures systemd.
PROFILE ?= quick
SCENARIO ?= ramp
FAULT_RATE ?= 0

perf-run:
	PROFILE="$(PROFILE)" SCENARIO="$(SCENARIO)" FAULT_RATE="$(FAULT_RATE)" \
		KUBECTL="$(KUBECTL)" K6_IMAGE="$(K6_IMAGE)" \
		sh "$(ROOT_DIR)/deploy/perf/run.sh"

perf-prepare:
	sh "$(ROOT_DIR)/deploy/perf/reset.sh" prepare

perf-restore:
	sh "$(ROOT_DIR)/deploy/perf/reset.sh" restore
