# deploy/server — OSV Server Deployment (172.20.2.48)

Triển khai OSV Platform lên server 172.20.2.48. Binary được **compile ở local** (Mac), sau đó **rsync lên server** và chạy trong container.

## Kiến trúc mạng

```
                     Internet
                         │
              ┌──────────▼──────────┐
              │ 103.67.184.32       │
              │ (172.20.2.16)       │
              │  nginx container    │  ← TLS termination (certbot)
              │  certbot container  │    domain: c12.openledger.vn
              └──────────┬──────────┘
                         │ proxy_pass http://172.20.2.48:8080
              ┌──────────▼──────────┐
              │ 172.20.2.48         │
              │  osv-server         │  ← pre-built binary
              │  postgres           │
              │  mongodb            │
              │  redis              │
              │  nats               │
              │  elasticsearch      │
              └─────────────────────┘
```

## Yêu cầu

- Mac (build machine): Go 1.26+, `rsync`, SSH access đến cả 2 server
- 172.20.2.48: Docker, Docker Compose, port 8080 (bind localhost only)
- 172.20.2.16: nginx container, certbot container, SSL cert cho `c12.openledger.vn`

## Lần đầu triển khai

### 1. Thiết lập SSH key
```bash
# Đảm bảo key có thể SSH vào cả 2 server
ssh-copy-id -i ~/.ssh/id_ed25519 ubuntu@172.20.2.48
ssh-copy-id -i ~/.ssh/id_ed25519 ubuntu@172.20.2.16
```

### 2. Tạo `.env` trên server 172.20.2.48
```bash
ssh ubuntu@172.20.2.48 'mkdir -p /opt/osv'
# Sau khi deploy lần đầu, .env.example sẽ được copy tự động
# Chỉnh sửa các giá trị secrets:
ssh ubuntu@172.20.2.48 'nano /opt/osv/.env'
```

### 3. Deploy nginx config lên 172.20.2.16
```bash
make deploy-nginx
```

### 4. Xin SSL cert (chạy trên 172.20.2.16 nếu chưa có)
```bash
ssh ubuntu@172.20.2.16
docker exec certbot certbot certonly \
  --webroot -w /var/www/certbot \
  -d c12.openledger.vn \
  --email your@email.com --agree-tos --non-interactive
docker exec nginx nginx -s reload
```

## Deploy thường ngày

```bash
# Full deploy (build + sync + restart + nginx config)
make deploy-full

# Chỉ deploy binary (không đụng nginx)
make deploy-server

# Chỉ cập nhật nginx config
make deploy-nginx

# Override SSH key hoặc user
make deploy-server SSH_KEY=~/.ssh/other_key DEPLOY_USER=admin
```

## Kiểm tra

```bash
# Health check qua local (trên 172.20.2.48)
curl http://localhost:8080/health

# Health check qua domain
curl https://c12.openledger.vn/health

# OSV API v1
curl https://c12.openledger.vn/v1/vulns/CVE-2021-44228

# Xem logs trên server
ssh ubuntu@172.20.2.48 'cd /opt/osv && docker compose logs -f osv-server'
```

## Files

| File | Mô tả |
|------|-------|
| `docker-compose.yml` | Docker Compose cho 172.20.2.48 — dùng binary pre-built |
| `.env.example` | Template biến môi trường (copy thành `.env` trên server) |
| `deploy.sh` | Script deploy tự động |
| `../nginx/c12.openledger.vn.conf` | Nginx reverse proxy config cho 172.20.2.16 |
