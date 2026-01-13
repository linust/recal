# ReCal Deployment Guide

## Quick Start - Local Development

### Option 1: Run directly (fastest)

```bash
# Build
go build -o recal ./cmd/recal

# Run with default config
./recal

# Or specify custom config
./recal -config=myconfig.yaml
```

Server will start at `http://localhost:8080`

### Option 2: Run with Docker Compose (recommended for production)

```bash
# Start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

### Option 3: Run with Docker directly

```bash
# Build image
docker build -t recal:latest .

# Run container
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v $(pwd)/data:/app/data \
  --name recal \
  recal:latest

# View logs
docker logs -f recal

# Stop container
docker stop recal && docker rm recal
```

## Production Deployment

### Step 1: Build the Binary or Docker Image

**Option A: Local Binary**
```bash
# Build for production (uses Docker for reproducibility)
make build

# Or build locally (faster, but not reproducible)
make build-local
```

**Option B: Docker Image**
```bash
# Build Docker image
docker build -t recal:latest .

# Or use GitHub Container Registry (automatic via CI/CD)
docker pull ghcr.io/your-username/recal:latest
```

### Step 2: Prepare Configuration

Copy and customize the config file:

```bash
cp config.yaml.example config.yaml
nano config.yaml
```

Key settings for production:
- `server.base_url`: Your public URL (e.g., `https://pb.thorsell.info`)
- `upstream.default_url`: Your upstream iCal feed URL
- `feeds.storage_path`: Path for named feed storage (default: `./data/feeds`)

### Step 3: Deploy to Your Server

**SSH to your server:**
```bash
ssh your-server
```

**Upload the binary or use Docker:**

#### Method 1: Binary Deployment
```bash
# Upload binary
scp recal your-server:/opt/recal/
scp config.yaml your-server:/opt/recal/

# On server
cd /opt/recal
chmod +x recal

# Create systemd service
sudo tee /etc/systemd/system/recal.service << 'EOF'
[Unit]
Description=ReCal Calendar Filter Service
After=network.target

[Service]
Type=simple
User=recal
WorkingDirectory=/opt/recal
ExecStart=/opt/recal/recal
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable recal
sudo systemctl start recal

# Check status
sudo systemctl status recal
```

#### Method 2: Docker Deployment

**Using Docker Compose (recommended):**

```bash
# On your server
cd /opt/recal
git pull  # or upload docker-compose.yml

# Start service
docker-compose up -d

# View logs
docker-compose logs -f

# Update to latest version
docker-compose pull
docker-compose up -d
```

**Using Docker Run:**

```bash
# Pull latest image from GitHub Container Registry
docker pull ghcr.io/your-username/recal:latest

# Stop old container
docker stop recal && docker rm recal

# Start new container
docker run -d \
  -p 8080:8080 \
  -v /opt/recal/config.yaml:/app/config.yaml:ro \
  -v /opt/recal/data:/app/data \
  --restart unless-stopped \
  --name recal \
  ghcr.io/your-username/recal:latest

# Check logs
docker logs -f recal
```

### Step 4: Configure Reverse Proxy (nginx)

```nginx
server {
    server_name pb.thorsell.info;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # SSL configuration (managed by certbot)
    listen 443 ssl;
    ssl_certificate /etc/letsencrypt/live/pb.thorsell.info/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pb.thorsell.info/privkey.pem;
}

server {
    if ($host = pb.thorsell.info) {
        return 301 https://$host$request_uri;
    }
    listen 80;
    server_name pb.thorsell.info;
    return 404;
}
```

Reload nginx:
```bash
sudo nginx -t
sudo systemctl reload nginx
```

## Updating Your Deployment

### For Binary Deployment

```bash
# On your local machine
make build
scp recal your-server:/opt/recal/

# On server
sudo systemctl restart recal
```

### For Docker Deployment

**Using GitHub Actions (Automatic):**

1. Push to `master` branch
2. GitHub Actions builds and pushes to `ghcr.io`
3. On your server, pull and restart:

```bash
cd /opt/recal
docker-compose pull
docker-compose up -d
```

**Manual Docker Update:**

```bash
# Build locally
docker build -t recal:latest .

# Save and transfer
docker save recal:latest | gzip > recal.tar.gz
scp recal.tar.gz your-server:

# On server
gunzip -c recal.tar.gz | docker load
docker-compose up -d
```

## Troubleshooting

### Check if service is running

```bash
# Binary deployment
sudo systemctl status recal
sudo journalctl -u recal -f

# Docker deployment
docker ps | grep recal
docker logs -f recal
```

### Test endpoints manually

```bash
# Health check
curl http://localhost:8080/health

# Test query endpoint
curl "http://localhost:8080/query?pattern=Test"

# Check admin page
curl -I http://localhost:8080/admin
```

### Common issues

**1. "Failed to load configuration"**
- Check config.yaml exists and is readable
- Verify YAML syntax with `yamllint config.yaml`

**2. "Address already in use"**
- Another service is using port 8080
- Change port in config.yaml or stop conflicting service

**3. Admin page shows old version**
- Clear browser cache (Ctrl+Shift+R)
- Verify service was restarted
- Check if reverse proxy is caching responses

**4. Docker container exits immediately**
- Check logs: `docker logs recal`
- Verify config file is mounted correctly
- Ensure data directory has write permissions

## Accessing the Admin Dashboard

Once deployed, access the admin interface at:

**https://your-domain.com/admin**

Features:
- Create/edit/delete named feeds
- Search through feeds (server-side search with pagination)
- View access statistics
- Copy feed URLs
- Preview filtered events
- Links to status and health endpoints

The admin dashboard supports 5000-10000+ feeds with:
- Server-side pagination (50 feeds per page)
- Debounced search (500ms delay)
- Sorted by most recently accessed

## Performance Tuning

For high-traffic deployments (5000+ feeds):

1. **Increase cache settings** in config.yaml:
```yaml
cache:
  max_size: 500       # More cache entries
  max_memory: 104857600  # 100MB
  default_ttl: 15m
```

2. **Use Docker with resource limits**:
```yaml
# docker-compose.yml
services:
  recal:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M
```

3. **Enable gzip compression** in nginx:
```nginx
gzip on;
gzip_types text/calendar application/json text/html;
```

4. **Monitor with Prometheus** (optional):
- Add metrics endpoint (future feature)
- Use `/health` for basic monitoring

## Backup and Recovery

**Backup feed data:**
```bash
# Feeds are stored in ./data/feeds/*.json
tar -czf feeds-backup-$(date +%Y%m%d).tar.gz data/feeds/

# Upload to backup location
scp feeds-backup-*.tar.gz backup-server:
```

**Restore from backup:**
```bash
tar -xzf feeds-backup-20260112.tar.gz
sudo systemctl restart recal  # or docker-compose restart
```

## Security Notes

- Admin endpoints (`/admin/feeds`) have no authentication by default
- Add authentication via reverse proxy (nginx basic auth or OAuth)
- Use HTTPS in production (Let's Encrypt)
- Keep Docker images updated
- Review feed permissions if storing sensitive data

Example nginx basic auth:
```nginx
location /admin {
    auth_basic "Admin Area";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}
```
