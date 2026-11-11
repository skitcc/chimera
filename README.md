# Chimera

Uncensorable music. Go API in `backend/`, Sound Chimera web client in `frontend/`.

## Run

```bash
cp .env.example .env
docker compose up --build
```

Welcome page: http://localhost:5173/  
Client: http://localhost:5173/legacy/  
API through the web server: http://localhost:5173/api/v1/tracks  
Swagger: http://localhost:5173/api/v1/docs/  
Service index: http://localhost:5173/management  
API process directly: http://127.0.0.1:8080

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
