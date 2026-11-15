# Build context is backend/: the gateway needs the contracts and platform modules next to it.
FROM golang:1.26-alpine AS build
ENV GOWORK=off
WORKDIR /src
COPY contracts ./contracts
COPY platform ./platform
COPY gateway ./gateway
RUN find . -name '*_test.go' -delete
WORKDIR /src/gateway
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -trimpath -o /gateway ./cmd/gateway

FROM alpine:3.21 AS production
RUN apk add --no-cache ca-certificates wget \
	&& adduser -D -H -u 10001 chimera
COPY --from=build /gateway /gateway
USER 10001
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD wget -qO- http://127.0.0.1:8080/live >/dev/null || exit 1
ENTRYPOINT ["/gateway"]
