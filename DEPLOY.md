# Deployment Guide

This guide covers deploying the ReCal application to your server using Docker.

## Overview

The deployment strategy keeps sensitive configuration (upstream calendar URLs, server endpoints) **outside** the Docker image for security. The configuration file is mounted at runtime.

## Prerequisites

- Docker installed on your server
- Docker Compose installed (optional, but recommended)
- Access to your Google Calendar iCal URL

## Setup Steps

### 1. Build the Docker Image

On your local machine or server:

```bash
# Build the image
docker build -t recal:latest .

# Or use Docker Compose to build
docker-compose build
```

The image contains:
- The compiled binary
- CA certificates
- Timezone data
- **NO configuration file** (for security)

### 2. Create Configuration File

Copy the example config and fill in your actual values:

```bash
cp config.yaml.example config.yaml
```

Edit `config.yaml` and set:

1. **server.base_url**: Your public server URL
   ```yaml
   server:
     base_url: "https://ical.yourdomain.com"
   ```

2. **upstream.default_url**: Your Google Calendar iCal URL
   ```yaml
   upstream:
     default_url: "https://calendar.google.com/calendar/ical/YOUR_ACTUAL_CALENDAR_ID%40group.calendar.google.com/public/basic.ics"
   ```

**Important**: Keep `config.yaml` private! Add it to `.gitignore` to prevent accidentally committing secrets.

### 3. Deploy with Docker Compose (Recommended)

```bash
# Start the service
docker-compose up -d

# Check logs
docker-compose logs -f

# Stop the service
docker-compose down
```

The `docker-compose.yml` automatically mounts `config.yaml` from your host.

### 4. Deploy with Plain Docker

If not using Docker Compose:

```bash
# Run with config mounted as volume
docker run -d \
  --name recal \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  --restart unless-stopped \
  --read-only \
  --security-opt no-new-privileges:true \
  --cap-drop ALL \
  recal:latest
```

**Flags explained**:
- `-v $(pwd)/config.yaml:/app/config.yaml:ro` - Mount config as read-only
- `--read-only` - Container filesystem is read-only (security)
- `--security-opt no-new-privileges:true` - Prevent privilege escalation
- `--cap-drop ALL` - Drop all Linux capabilities (minimal permissions)

### 5. Verify Deployment

```bash
# Health check
curl http://localhost:8080/health

# Status endpoint
curl http://localhost:8080/status

# Test filtering (replace with your actual filter params)
curl "http://localhost:8080/filter?Grad=4"
```

## Updating Configuration

To update configuration without rebuilding the image:

```bash
# 1. Edit config.yaml on the host
vim config.yaml

# 2. Restart the container
docker-compose restart

# Or with plain Docker:
docker restart recal
```

The config file is read on startup, so a restart picks up changes immediately.

## Transferring to Server

### Option 1: Build on Server (Recommended)

```bash
# On your server
git clone <your-repo-url>
cd recal

# Copy example config and fill in values
cp config.yaml.example config.yaml
vim config.yaml  # Add your secrets

# Build and run
docker-compose up -d
```

### Option 2: Transfer Pre-built Image

If you want to build locally and transfer:

```bash
# On local machine - build and save
docker build -t recal:latest .
docker save recal:latest | gzip > recal.tar.gz

# Transfer to server
scp recal.tar.gz user@server:/path/to/deploy/
scp config.yaml user@server:/path/to/deploy/
scp docker-compose.yml user@server:/path/to/deploy/

# On server - load and run
docker load < recal.tar.gz
docker-compose up -d
```

## Reverse Proxy Setup

For production, use a reverse proxy (nginx, Caddy, Traefik) for:
- HTTPS termination
- Custom domain
- Rate limiting
- Access logs

### Example: Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name ical.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/ical.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/ical.yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Cache static responses
        proxy_cache_valid 200 15m;
    }
}
```

### Example: Caddy

```
ical.yourdomain.com {
    reverse_proxy localhost:8080
}
```

Caddy automatically handles HTTPS via Let's Encrypt!

## Security Best Practices

1. **Keep config.yaml private**
   - Never commit to git
   - Restrict file permissions: `chmod 600 config.yaml`
   - Use separate configs for dev/staging/prod

2. **Run behind reverse proxy**
   - Use HTTPS in production
   - Implement rate limiting at proxy level
   - Hide internal port (only expose 443)

3. **Monitor logs**
   ```bash
   docker-compose logs -f recal
   ```

4. **Regular updates**
   - Rebuild image when Go or Alpine updates are available
   - Check for dependency vulnerabilities: `go list -m -u all`

## Troubleshooting

### Container won't start

```bash
# Check logs
docker-compose logs recal

# Common issues:
# 1. config.yaml not found - ensure it exists in the same directory
# 2. Port 8080 already in use - change port in docker-compose.yml
# 3. Permission denied - check config.yaml permissions
```

### Config changes not taking effect

```bash
# Restart container to reload config
docker-compose restart
```

### Cannot reach service

```bash
# Verify port is exposed
docker ps

# Check if service is listening
curl http://localhost:8080/health

# If using reverse proxy, check proxy logs
```

## Maintenance

### View logs

```bash
docker-compose logs -f
```

### Restart service

```bash
docker-compose restart
```

### Update application

```bash
# Pull latest code
git pull

# Rebuild image
docker-compose build

# Recreate container with new image
docker-compose up -d
```

### Backup

Only `config.yaml` needs backing up (contains secrets):

```bash
# Backup config
cp config.yaml config.yaml.backup

# Or include in server backup
tar czf recal-backup.tar.gz config.yaml docker-compose.yml
```

## Docker Image Details

**Base image**: `gcr.io/distroless/static-debian12:nonroot`
- Minimal attack surface (no shell, no package manager)
- Non-root user (UID 65532)
- Static binary (no runtime dependencies)

**Image size**: ~18MB (compressed)

**Security features**:
- Read-only filesystem
- Dropped Linux capabilities
- No privilege escalation
- Distroless base (minimal attack surface)

## Alternative: systemd Service

If you prefer running without Docker:

```bash
# Build binary locally
make build

# Copy to /usr/local/bin
sudo cp recal /usr/local/bin/

# Create config directory
sudo mkdir -p /etc/recal
sudo cp config.yaml /etc/recal/

# Create systemd service
sudo tee /etc/systemd/system/recal.service <<EOF
[Unit]
Description=ReCal Service
After=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
WorkingDirectory=/etc/recal
ExecStart=/usr/local/bin/recal
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable recal
sudo systemctl start recal
sudo systemctl status recal
```

## Support

For issues or questions:
- Check logs: `docker-compose logs -f`
- Review config: `cat config.yaml` (but hide secrets!)
- Test upstream: `curl "<your-upstream-url>"`
- Test health: `curl http://localhost:8080/health`
