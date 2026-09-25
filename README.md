# Chimera

Uncensorable music. Go API in `backend/`, Sound Chimera web client in `frontend/`.

## Run

```bash
cp .env.example .env
docker compose up --build
```

UI: http://localhost:5173  
API: http://127.0.0.1:8080  
Swagger: http://localhost:5173/swagger/index.html

## Backend tests

Backend test tooling is Docker-based; no Go or PostgreSQL installation is
required on the host.

```bash
cd backend
make test
make test-integration
make test-all
```

See [backend/TESTING.md](backend/TESTING.md) for offline runs, shuffle seed
reproduction, coverage reports, Allure generation, and test conventions.
