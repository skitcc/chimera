SHELL := /bin/sh

ROOT_DIR := $(abspath ..)
ARTIFACTS_DIR ?= $(CURDIR)
TEST_IMAGE ?= chimera-backend-test:local
GOBCO_VERSION ?= v1.3.4
ALLURE_IMAGE ?= tobix/allure-cli:2
ALLURE_PORT ?= 5252
NGINX_IMAGE ?= nginx:1.27-alpine
COMPOSE_PROJECT_NAME ?= chimera-tests
COMPOSE := docker compose --project-name $(COMPOSE_PROJECT_NAME) --project-directory $(ROOT_DIR) -f $(ROOT_DIR)/docker-compose.test.yml

TEST_PACKAGES ?= ./...
INTEGRATION_PACKAGES ?= ./...
INTEGRATION_TAGS ?= integration
COVERAGE_PACKAGES ?= ./internal/domain ./internal/usecase ./internal/infra/adapters/postgres
GOBCO_PACKAGES ?= ./internal/domain ./internal/usecase ./internal/infra/adapters/postgres
GO_TEST_P ?= 1
GO_TEST_PARALLEL ?= 4
SHUFFLE_SEED ?= on
TEST_UID := $(shell id -u)
TEST_GID := $(shell id -g)

RUN_TEST_IMAGE = docker run --rm --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp -e ALLURE_RESULTS_DIR=/artifacts/allure-results -v "$(ARTIFACTS_DIR):/artifacts" "$(TEST_IMAGE)"
GO_TEST_FLAGS = -count=1 -p=$(GO_TEST_P) -parallel=$(GO_TEST_PARALLEL)

.PHONY: help test-image test test-shuffle test-offline test-integration test-all coverage gobco-condition gobco-branch allure-results allure-report allure-open clean-test-results
.NOTPARALLEL: test-all

help:
	@printf '%s\n' \
		'test-image       Build the reusable Docker test image' \
		'test             Run the standard test suite' \
		'test-shuffle     Run tests with a recorded/random shuffle seed' \
		'test-offline     Run the prebuilt image without network access' \
		'test-integration Run integration-tagged tests with ephemeral PostgreSQL' \
		'test-all         Run tests, coverage, gobco, and integration tests' \
		'coverage         Write statement profile and HTML coverage' \
		'gobco-condition  Write gobco condition coverage output' \
		'gobco-branch     Write gobco branch coverage output' \
		'allure-results   Run tests configured to emit Allure result files' \
		'allure-report    Generate an Allure HTML report in Docker' \
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

test-integration: test-image | $(ARTIFACTS_DIR)
	@set -eu; \
		TEST_IMAGE="$(TEST_IMAGE)" TEST_UID="$(TEST_UID)" TEST_GID="$(TEST_GID)" $(COMPOSE) up --detach --wait postgres-test; \
		trap 'TEST_IMAGE="$(TEST_IMAGE)" TEST_UID="$(TEST_UID)" TEST_GID="$(TEST_GID)" $(COMPOSE) down --volumes --remove-orphans' EXIT INT TERM; \
		TEST_IMAGE="$(TEST_IMAGE)" TEST_UID="$(TEST_UID)" TEST_GID="$(TEST_GID)" $(COMPOSE) run --rm --no-deps test-runner \
			go test $(GO_TEST_FLAGS) -tags="$(INTEGRATION_TAGS)" $(INTEGRATION_PACKAGES)

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

allure-report: allure-results
	rm -rf "$(ARTIFACTS_DIR)/allure-report"
	mkdir -p "$(ARTIFACTS_DIR)/allure-report"
	docker run --rm --user $(TEST_UID):$(TEST_GID) -e HOME=/tmp \
		-v "$(ARTIFACTS_DIR):/work" "$(ALLURE_IMAGE)" \
		generate /work/allure-results --clean -o /work/allure-report

allure-open:
	@test -f "$(ARTIFACTS_DIR)/allure-report/index.html" || { printf '%s\n' 'report is missing; run make allure-report first' >&2; exit 1; }
	@docker image inspect "$(NGINX_IMAGE)" >/dev/null 2>&1 || docker pull "$(NGINX_IMAGE)"
	@printf '%s\n' "serving Allure report at http://127.0.0.1:$(ALLURE_PORT)"
	docker run --rm --pull=never -p "$(ALLURE_PORT):80" \
		-v "$(ARTIFACTS_DIR)/allure-report:/usr/share/nginx/html:ro" \
		"$(NGINX_IMAGE)"

test-all: test test-shuffle test-offline coverage gobco-condition gobco-branch test-integration

clean-test-results:
	rm -rf "$(ARTIFACTS_DIR)/allure-results" "$(ARTIFACTS_DIR)/allure-report"
	rm -f "$(ARTIFACTS_DIR)/allure-report.html"
	rm -f "$(ARTIFACTS_DIR)/coverage.out" "$(ARTIFACTS_DIR)/coverage.html"
	rm -f "$(ARTIFACTS_DIR)/condition-coverage.txt" "$(ARTIFACTS_DIR)/branch-coverage.txt"
