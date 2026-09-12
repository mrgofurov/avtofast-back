# AvtoFast Production Deployment Methodology

This guide explains the step-by-step deployment methodology for hosting the **AvtoFast** backend on an 8-core CPU / 8GB RAM / 100GB SSD production node at `api.avtotest.uz`.

---

## 1. Overview of the Deployment Pipeline

```
[Git Push to main]
       │
       ▼
[GitHub Actions CI] ───► Run Tests with Race Detector & Benchmarks
       │
       ▼
[Build Multi-stage Docker Image] ───► Push to GitHub Container Registry (ghcr.io)
       │
       ▼
[Deploy via SSH to api.avtotest.uz]
       │
       ├─► 1. Pull latest image from GHCR
       ├─► 2. Run Database Migrations (`bin/migrate -dir=up`)
       ├─► 3. Perform Zero-Downtime Rolling Update (`docker compose up -d --no-deps api`)
       ├─► 4. Health Check Verification (`GET /healthz`)
       └─► 5. Rollback on failure
```

---

## 2. Server Prerequisites (One-Time Setup)

### 2.1. DNS Configuration
Point the A record of `api.avtotest.uz` to your production server IP:
```
Type: A
Host: api
Domain: avtotest.uz
Value: <YOUR_SERVER_IP>
TTL: 300
```

### 2.2. Install Docker & Docker Compose
On your Ubuntu/Debian production node:
```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl gnupg
# Install Docker and Compose plugin
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

### 2.3. Setup Directory & Environment
Create the deployment directory on the server:
```bash
sudo mkdir -p /opt/avtofast
sudo chown -R $USER:$USER /opt/avtofast
cd /opt/avtofast
```

Copy `docker-compose.prod.yml` and `.env.example` into `/opt/avtofast/`:
```bash
cp docker-compose.prod.yml /opt/avtofast/
cp .env.example /opt/avtofast/.env
```
Edit `/opt/avtofast/.env` and supply production values:
- `POSTGRES_PASSWORD=<strong_random_password>`
- `JWT_SECRET=<strong_jwt_secret>`
- `API_BASE_URL=https://api.avtotest.uz/v1`

### 2.4. SSL Certificate Setup via Certbot & Nginx
Install Certbot and Nginx:
```bash
sudo apt-get install -y nginx certbot python3-certbot-nginx

# Obtain SSL Certificate for api.avtotest.uz
sudo certbot --nginx -d api.avtotest.uz
```
Copy `deploy/nginx.conf` to `/etc/nginx/sites-available/api.avtotest.uz`:
```bash
sudo cp deploy/nginx.conf /etc/nginx/sites-available/api.avtotest.uz
sudo ln -sf /etc/nginx/sites-available/api.avtotest.uz /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

---

## 3. GitHub Secrets Configuration

In your GitHub repository, go to **Settings ➔ Secrets and variables ➔ Actions ➔ New repository secret** and add:

| Secret Name | Description | Example |
| --- | --- | --- |
| `SERVER_HOST` | Production server IP or hostname | `api.avtotest.uz` or `123.45.67.89` |
| `SERVER_USER` | SSH user on the production server | `root` or `deploy` |
| `SSH_PRIVATE_KEY` | Private SSH key authorized in `~/.ssh/authorized_keys` | `-----BEGIN OPENSSH PRIVATE KEY-----...` |
| `SSH_PORT` | SSH port (defaults to 22 if omitted) | `22` |

> Note: GitHub provides `${{ secrets.GITHUB_TOKEN }}` automatically with read/write access to GitHub Container Registry (GHCR).

---

## 4. Initial Launch & Seeding

On the production server, start Postgres and Redis first:
```bash
cd /opt/avtofast
docker compose -f docker-compose.prod.yml up -d postgres redis
```

Run initial schema migration:
```bash
docker compose -f docker-compose.prod.yml run --rm api /app/bin/migrate -dir=up
```

Seed initial question pack and 4-locale translations:
```bash
docker compose -f docker-compose.prod.yml run --rm api /app/bin/seed
```

Start the API service:
```bash
docker compose -f docker-compose.prod.yml up -d api
```

Verify everything is up and responding:
```bash
curl -I https://api.avtotest.uz/healthz
curl https://api.avtotest.uz/v1/bootstrap
```

---

## 5. Automated Deployments Workflow

Once the above steps are in place:
1. Push any changes to the `main` branch or create a Git tag (`v1.0.0`).
2. GitHub Actions will:
   - Run tests with Go's race detector.
   - Run benchmarks.
   - Build and publish the Docker image to GHCR.
   - SSH into `api.avtotest.uz`.
   - Apply any new migrations in `migrations/`.
   - Perform a rolling restart of the API container without terminating active connections.
   - Verify `http://127.0.0.1:8080/healthz`.

---

## 6. Zero-Downtime & Rollback Methodology

### Zero-Downtime Updates
Fiber handles `SIGTERM` signals gracefully, allowing existing in-flight HTTP requests up to 5 seconds to complete before terminating. The Nginx reverse proxy buffers incoming requests during the 1-2 seconds it takes Docker to switch containers.

### Emergency Rollback
If a deployment fails or a bug is detected:
```bash
# 1. Roll back to previous container image version:
docker compose -f docker-compose.prod.yml up -d --no-deps api:sha-<previous_commit_sha>

# 2. If a database rollback is required:
docker run --rm --network avtofast-back_default --env-file .env \
  ghcr.io/avtofast/avtofast-back:latest /app/bin/migrate -dir=down
```
