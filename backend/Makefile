SHELL := /bin/sh

ROOT_DIR := $(abspath ..)
ARTIFACTS_DIR ?= $(CURDIR)
TEST_IMAGE ?= chimera-backend-test:local
GOBCO_VERSION ?= v1.3.4
ALLURE_IMAGE ?= tobix/allure-cli:2
ALLURE_PORT ?= 5252
NGINX_IMAGE ?= nginx:1.27-alpine
API_IMAGE ?= chimera-stand-api:local
NETSHOOT_IMAGE ?= nicolaka/netshoot:v0.13
E2E_OUT ?= $(ARTIFACTS_DIR)/e2e-out

TEST_PACKAGES ?= ./...
INTEGRATION_PACKAGES ?= ./internal/infra/adapters/postgres ./internal/usecase
# INTEGRATION_TAGS — build tag файлов с //go:build integration.
# INTEGRATION_RUN — регулярка go test -run.XXxTest
INTEGRATION_TAGS ?= integration
INTEGRATION_RUN ?= Integration
POSTGRES_TEST_IMAGE ?= postgres:17-alpine
COVERAGE_PACKAGES ?= ./internal/domain ./internal/usecase ./internal/infra/adapters/postgres
GOBCO_PACKAGES ?= ./internal/domain ./internal/usecase ./internal/infra/adapters/postgres
# GO_TEST_P — сколько пакетов go test запускает сразу (-p).
# GO_TEST_PARALLEL — сколько тестов с t.Parallel() идёт внутри пакета (-parallel).
GO_TEST_P ?= 1
GO_TEST_PARALLEL ?= 4
SHUFFLE_SEED ?= on
TEST_UID := $(shell id -u)
TEST_GID := $(shell id -g)

RUN_TEST_IMAGE = docker run --rm --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp -e ALLURE_RESULTS_DIR=/artifacts/allure-results -v "$(ARTIFACTS_DIR):/artifacts" "$(TEST_IMAGE)"
GO_TEST_FLAGS = -count=1 -p=$(GO_TEST_P) -parallel=$(GO_TEST_PARALLEL)

.PHONY: help test-image test test-shuffle test-offline stand-up stand-refresh stand-gc stand-down test-integration test-e2e demo-capture test-all coverage gobco-condition gobco-branch allure-results allure-meta allure-render allure-report allure-open spec-matrix spec-check clean-test-results
.NOTPARALLEL: test-all

help:
	@printf '%s\n' \
		'test-image       Build the reusable Docker test image' \
		'test             Run the standard test suite' \
		'test-shuffle     Run tests with a recorded/random shuffle seed' \
		'test-offline     Run the prebuilt image without network access' \
		'stand-up         Start the shared Postgres and RustFS stand' \
		'stand-refresh    Rebuild chimera_template from schema.sql' \
		'stand-gc         Drop t_* databases that have no sessions' \
		'stand-down       Stop the stand without deleting its volumes' \
		'test-integration Run integration tests against the running stand' \
		'test-e2e         Run the demo against a private API on the stand' \
		'demo-capture     Replay the demo with curl and write pcap captures' \
		'test-all         Run unit suites, coverage, gobco, and integration tests' \
		'coverage         Write statement profile and HTML coverage' \
		'gobco-condition  Write gobco condition coverage output' \
		'gobco-branch     Write gobco branch coverage output' \
		'allure-results   Run unit, integration, and e2e tests into allure-results' \
		'allure-render    Render allure-report from existing results (no test run)' \
		'allure-report    Run allure-results, then allure-render' \
		'spec-matrix      Rewrite the TEST_SPEC.md matrix from allure-results' \
		'spec-check       Fail when the TEST_SPEC.md matrix differs from allure-results' \
		'allure-open      Serve the report at http://127.0.0.1:$(ALLURE_PORT)'

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

stand-refresh:
	sh scripts/stand-refresh.sh

stand-gc:
	sh scripts/stand-gc.sh

stand-down:
	sh scripts/stand-down.sh

test-integration: | $(ARTIFACTS_DIR)
	@test -f "$(ROOT_DIR)/.env" || { printf '%s\n' 'missing ../.env; copy .env.example to .env' >&2; exit 1; }
	STAND_ALLURE_SUITE=integration sh scripts/stand-require.sh
	$(MAKE) test-image
	ARTIFACTS_DIR="$(ARTIFACTS_DIR)" TEST_IMAGE="$(TEST_IMAGE)" TEST_UID="$(TEST_UID)" TEST_GID="$(TEST_GID)" \
		INTEGRATION_TAGS="$(INTEGRATION_TAGS)" INTEGRATION_RUN="$(INTEGRATION_RUN)" \
		sh scripts/test-integration.sh $(INTEGRATION_PACKAGES)

test-e2e: | $(ARTIFACTS_DIR)
	@test -f "$(ROOT_DIR)/.env" || { printf '%s\n' 'missing ../.env; copy .env.example to .env' >&2; exit 1; }
	STAND_ALLURE_SUITE=e2e sh scripts/stand-require.sh rustfs
	$(MAKE) test-image
	docker build -f Dockerfile --tag "$(API_IMAGE)" .
	ARTIFACTS_DIR="$(ARTIFACTS_DIR)" TEST_IMAGE="$(TEST_IMAGE)" API_IMAGE="$(API_IMAGE)" \
		TEST_UID="$(TEST_UID)" TEST_GID="$(TEST_GID)" \
		sh scripts/stand-e2e.sh

demo-capture:
	NETSHOOT_IMAGE="$(NETSHOOT_IMAGE)" sh scripts/demo-capture.sh

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
		printf 'Suites=unit (domain, usecase, postgres stub and pgxmock), integration (one PostgreSQL clone per run), e2e (stand API)\n'; \
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
	rm -rf "$(ARTIFACTS_DIR)/allure-results" "$(ARTIFACTS_DIR)/allure-report" "$(ARTIFACTS_DIR)/e2e-out"
	rm -f "$(ARTIFACTS_DIR)/allure-report.html"
	rm -f "$(ARTIFACTS_DIR)/coverage.out" "$(ARTIFACTS_DIR)/coverage.html"
	rm -f "$(ARTIFACTS_DIR)/condition-coverage.txt" "$(ARTIFACTS_DIR)/branch-coverage.txt"
