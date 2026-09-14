# Chimera

Uncensorable music. Go API in `backend/`, Sound Chimera web client in `frontend/`.

## Run

```bash
docker compose up -d
cd backend && go run ./cmd/api
cd frontend && npm install && npm run dev
```

UI: http://localhost:5173  
API: http://127.0.0.1:8080  
Swagger: http://127.0.0.1:8080/swagger/index.html
