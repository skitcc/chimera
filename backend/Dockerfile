FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY docs ./docs
COPY internal ./internal
# Test code and test-only libraries never enter the production build graph.
RUN find . -name '*_test.go' -delete && rm -rf internal/testkit
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -trimpath -o /api ./cmd/api
# Fail the build if a test-only module (see go.mod) ever gets linked into the API.
RUN if go version -m /api | grep -E 'allure-framework|pashagolub/pgxmock'; then \
		echo 'test-only module linked into production binary' >&2; exit 1; fi

FROM alpine:3.21 AS production
RUN apk add --no-cache ca-certificates wget \
	&& adduser -D -H chimera
COPY --from=build /api /api
USER chimera
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD wget -qO- http://127.0.0.1:8080/live >/dev/null || exit 1
ENTRYPOINT ["/api"]
