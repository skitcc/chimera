# Build context is backend/: the service needs the contracts and platform modules next to it.
FROM golang:1.26-alpine AS build
ENV GOWORK=off
WORKDIR /src
COPY contracts ./contracts
COPY platform ./platform
# Only the module file: go.mod lists testkit for tests, the binary never imports it.
COPY testkit/go.mod testkit/go.sum ./testkit/
COPY catalog ./catalog
# Test code and test-only libraries never enter the production build graph.
RUN find . -name '*_test.go' -delete && rm -rf catalog/internal/testkit
WORKDIR /src/catalog
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -trimpath -o /service ./cmd/catalog
RUN if go version -m /service | grep -E 'allure-framework|pashagolub/pgxmock'; then \
		echo 'test-only module linked into production binary' >&2; exit 1; fi

FROM alpine:3.21 AS production
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 chimera
COPY --from=build /service /service
USER 10001
EXPOSE 9090
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD ["/service", "-healthcheck"]
ENTRYPOINT ["/service"]
