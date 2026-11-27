# Chimera

Uncensorable music. Go services in `backend/`, Sound Chimera web client in `frontend/`.

The backend is four services in one Go workspace (`backend/go.work`): Identity
(accounts, sign-in, JWT issuing), Catalog (tracks and files), Social (likes,
follows, playlists) and Gateway (the public REST API). They talk gRPC inside
and exchange deletion events through Kafka. See
[docs/architecture.md](docs/architecture.md).

## Run

```bash
cp .env.example .env
docker compose up --build
```

UI: http://localhost:5173  
API (Gateway): http://127.0.0.1:8080  
Swagger: http://localhost:5173/swagger/index.html

An `.env` from before the split lacks the per-service database passwords and
Kafka settings: compare it with `.env.example`. The JWT key pair is generated
into Docker volumes on first start.

## Run in minikube

```bash
cd backend
make k8s-images   # build the images into the minikube node
make k8s-up       # keys, secrets.env, ingress addon, apply, wait
make k8s-e2e      # the demo scenario through the ingress
```

UI: http://chimera.192.168.49.2.nip.io  
`make k8s-status`, `make k8s-logs SVC=catalog`, `make k8s-down`.

## Backend tests

Backend test tooling is Docker-based; no Go or PostgreSQL installation is
required on the host.

```bash
cd backend
make test
make stand-up && make test-integration
make test-e2e
make test-all
```

See [backend/TESTING.md](backend/TESTING.md) for offline runs, shuffle seed
reproduction, coverage reports, Allure generation, and test conventions.

Аутентификация, SMTP и BDD E2E: [docs/auth-security.md](docs/auth-security.md).
