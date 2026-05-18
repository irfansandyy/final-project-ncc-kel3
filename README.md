# Production-Ready Llama Chat App

This repository contains a full-stack chat application with:

- Frontend: Next.js App Router
- Backend: Go REST API
- Database: PostgreSQL
- Local LLM Inference: Docker Model Runner using Hugging Face model source
- Reverse Proxy: Nginx (self-signed or custom certs)

Nginx runs in Docker and terminates TLS using certificates mounted from `nginx/certs`.

The application supports JWT authentication, per-user persistent chat history, creating new chats, and AI responses using `hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K` via Docker Model Runner.

## 1. Project Structure

```text
.
├── backend/
│   ├── config/
│   ├── handlers/
│   ├── middleware/
│   ├── models/
│   ├── repositories/
│   ├── services/
│   ├── Dockerfile
│   └── main.go
├── frontend/
│   ├── app/
│   ├── components/
│   ├── lib/
│   └── Dockerfile
├── database/
│   └── init/
│       └── 001_init.sql
├── nginx/
│   └── templates/
│       └── app.conf.template
├── docker-compose.yml
└── .env.example
```

## 2. Core Features

- User registration and login (`/api/auth/register`, `/api/auth/login`)
- JWT authentication middleware for protected chat routes
- Password hashing using bcrypt
- Persistent chat sessions per user in PostgreSQL
- New chat creation endpoint (`POST /api/chats`)
- Message persistence for both user and AI
- Local LLM integration with Docker Model Runner

## 3. LLM Requirements Compliance

- Model family: `meta-llama/Llama-3.2-1B-Instruct`
- Runtime: Docker Model Runner (OpenAI-compatible endpoint)
- Model source: Hugging Face (`hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K`)
- Context limiter: `LLM_CTX_SIZE=4096` enforced in backend request assembly
- No model reload per request:
  - Model is loaded and managed by Docker Model Runner.
  - Backend uses a singleton LLM client (`services.GetLLMService`) that reuses the same HTTP client and base URL.

### Default startup flow (Docker Model Runner)

```bash
export HF_TOKEN=$(cat ~/.cache/huggingface/token)
./scripts/docker-model-run.sh hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K
```
If your machine has limited RAM/VRAM, use the lower-memory quantization:

```bash
./scripts/docker-model-run.sh hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q4_K_M
```

The startup script now also auto-fallbacks from `Q6_K` to `Q4_K_M` when model initialization fails (can be disabled with `AUTO_FALLBACK_LOW_MEM=0`).
It also unloads previously running models before startup to avoid hidden memory contention (can be disabled with `UNLOAD_EXISTING_MODELS=0`).

Alternative login method:

```bash
env PATH="$HOME/.local/bin:$PATH" hf auth login
```

If you are using an IP address (no public domain), create a self-signed cert:

```bash
mkdir -p nginx/certs
DOMAIN=23.100.94.231
openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
  -keyout nginx/certs/privkey.pem \
  -out nginx/certs/fullchain.pem \
  -subj "/CN=${DOMAIN}" \
  -addext "subjectAltName=IP:${DOMAIN}"
```

If you already have a certificate, place it here:

- `nginx/certs/fullchain.pem`
- `nginx/certs/privkey.pem`

Then start the application stack (includes Nginx):

```bash
docker compose --env-file .env up -d --build
```

Or run the default one-command bootstrap:

```bash
./scripts/up-with-dmr.sh
```

Nginx config for the Docker reverse proxy lives in `nginx/templates/app.conf.template`.

Default `.env` values are already configured for this flow:

```bash
LLM_BASE_URL=http://model-runner.docker.internal:12434/engines/v1
LLM_MODEL_NAME=hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K
LLM_CTX_SIZE=4096
```
Important memory note:

- `LLM_CTX_SIZE` in this project limits backend prompt assembly.
- Increasing it (for example to `16384`) increases runtime KV cache pressure during inference and can cause out-of-memory on smaller machines.
- If you get `inference backend took too long to initialize`, use a smaller quantization (`Q4_K_M`) and keep context in the `2048-4096` range.

With this setup, backend calls Docker Model Runner OpenAI-compatible endpoint at:

```text
http://model-runner.docker.internal:12434/engines/v1/chat/completions
```

## 4. API Endpoints

### Public

- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /health`

### Protected (JWT Bearer)

- `GET /api/chats`
- `POST /api/chats`
- `GET /api/chats/{chatID}/messages`
- `POST /api/chats/{chatID}/messages`

## 5. Health Endpoint

### Route

- `GET /health`

### Success Response (extended)

```json
{
  "status": "ok",
  "services": {
    "database": "ok",
    "llm": "ok"
  }
}
```

### Simple Response

```text
GET /health?simple=true
```

Returns:

```json
{
  "status": "ok"
}
```

### What is checked

- Database readiness via `db.PingContext`
- LLM readiness via lightweight Docker Model Runner model check (`/v1/models`)

The endpoint always responds with HTTP 200 and reports service state details.

## 6. Environment Setup

1. Copy environment template:

```bash
cp .env.example .env
```

1. Configure reverse proxy domain or IP in `.env`:

```bash
DOMAIN=23.100.94.231
```

For local testing without public DNS, keep `DOMAIN=localhost`.

1. Start Docker Model Runner model:

```bash
hf auth login
./scripts/docker-model-run.sh hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K
```

## 7. Run With Docker Compose

```bash
docker compose --env-file .env up -d --build
```

Check status:

```bash
docker compose ps
```

Verify health endpoint:

```bash
curl -k https://localhost/health
```

Application URL:

```text
https://localhost
```

All public traffic is served through the reverse proxy on port 443.

## 8. Local Development (Without Docker)

### Backend

```bash
cd backend
go mod tidy
go run .
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

## 9. Production Deployment to DigitalOcean VPS

1. Provision an Ubuntu droplet (recommended minimum 4 vCPU / 8 GB RAM).
1. SSH into VPS and install Docker + Compose plugin.
1. Clone repository on VPS.
1. Create `.env` from `.env.example` and set secure values: strong `JWT_SECRET`, and `DOMAIN` set to your public IP or domain (must match your TLS certificate).
1. On VPS, authenticate Hugging Face and start the Docker Model Runner model:

```bash
hf auth login
./scripts/docker-model-run.sh hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q6_K
```

1. Create or copy your TLS certs into `nginx/certs` as `fullchain.pem` and `privkey.pem`.

1. Start services:

```bash
docker compose --env-file .env up -d --build
```

1. Open firewall ports (DigitalOcean Cloud Firewall + host UFW): `22/tcp`, `80/tcp` (redirect), and `443/tcp` (public HTTPS).
  To enforce this on the host with UFW:

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```
1. Verify external health check:

```bash
curl -k https://<YOUR_DOMAIN>/health
```

Routing is handled by the Nginx container:

- `/` to frontend service
- `/api/*` and `/health` to backend service


## 10. Performance Notes

- CPU-friendly local inference via Docker Model Runner
- Connection pooling for PostgreSQL in backend
- Centralized singleton LLM client in backend
- Lightweight health checks
- Graceful shutdown in backend server
- Basic request rate limiting middleware

## 11. CI/CD With Jenkins + SonarQube

### Start the CI stack

1. Create the shared Docker network:

```bash
docker network create jenkins
```

1. Build and run Jenkins + SonarQube:

```bash
docker compose -f docker-compose.jenkins-sonarqube.yml up -d --build
```

1. Open the services:

- Jenkins: `http://127.0.0.1:8080/jenkins`
- SonarQube: `http://127.0.0.1:9000/sonarqube`

Default SonarQube login is `admin` / `admin` (you will be asked to change it).

### Jenkins setup (one time)

1. Unlock Jenkins using `/var/jenkins_home/secrets/initialAdminPassword`.
1. Install recommended plugins (GitHub and SonarQube Scanner are required).
1. Manage Jenkins -> System:
  - Add SonarQube server with name `SonarQube`.
  - Server URL: `http://sonarqube:9000/sonarqube`.
  - Add a token credential (Secret text) from SonarQube.
1. Manage Jenkins -> Tools:
  - Add SonarQube Scanner tool named `SonarQube Scanner`.
1. Create a Pipeline job that uses the repository Jenkinsfile.
1. If the repository is private, set `GIT_CREDENTIALS_ID` to a Jenkins credential.

### Webhook trigger

- In Jenkins job settings: enable GitHub hook trigger for GITScm polling.
- In GitHub repo settings: add webhook URL `http(s)://<jenkins-host>/jenkins/github-webhook/`.

### Build badge

Use the Jenkins badge URL (replace placeholders):

```text
http(s)://<jenkins-host>/jenkins/job/<job-name>/badge/icon
```
