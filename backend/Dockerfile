FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api

FROM golang:1.26-alpine AS test-tools
ARG GOBCO_VERSION=v1.3.4
RUN GOBIN=/tools go install github.com/rillig/gobco@${GOBCO_VERSION}

FROM golang:1.26-alpine AS test
RUN apk add --no-cache build-base ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=test-tools /tools/gobco /usr/local/bin/gobco
ENV GOCACHE=/tmp/go-build \
	GOTMPDIR=/tmp/go-tmp
RUN mkdir -p /artifacts /tmp/go-build /tmp/go-tmp \
	&& chmod 1777 /artifacts /tmp/go-build /tmp/go-tmp
CMD ["go", "test", "./..."]

FROM alpine:3.21 AS production
RUN apk add --no-cache ca-certificates wget \
	&& adduser -D -H chimera
COPY --from=build /api /api
USER chimera
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD wget -qO- http://127.0.0.1:8080/live >/dev/null || exit 1
ENTRYPOINT ["/api"]
