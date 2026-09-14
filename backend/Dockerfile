FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget \
	&& adduser -D -H chimera
COPY --from=build /api /api
USER chimera
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD wget -qO- http://127.0.0.1:8080/live >/dev/null || exit 1
ENTRYPOINT ["/api"]
