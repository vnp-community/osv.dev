#!/bin/bash
set -e

echo "=================================================="
echo "    OSV GATEWAY DEPLOYMENT (to 103.67.184.32)     "
echo "=================================================="

GATEWAY_IP="172.20.2.16"
DOMAIN="c12.openledger.vn"
CONF_SRC="c12.openledger.vn.conf"
SSH_USER="ubuntu"

# 1. Sync config
echo "[1/3] Syncing Nginx config to proxy server..."
scp $CONF_SRC ${SSH_USER}@${GATEWAY_IP}:/tmp/$CONF_SRC

# 2. Run certbot bootstrap and deploy on remote
echo "[2/3] Configuring Nginx and Let's Encrypt on server..."
ssh ${SSH_USER}@${GATEWAY_IP} << 'EOF'
  DOMAIN="c12.openledger.vn"
  REMOTE_CONF_DIR="/home/ubuntu/vnp-qa-platform/proxy/conf.d"
  
  echo "Checking existing certificates for $DOMAIN..."
  if docker run --rm -v proxy_certbot-conf:/etc/letsencrypt alpine sh -c "ls /etc/letsencrypt/live/$DOMAIN/fullchain.pem" >/dev/null 2>&1; then
    echo "✅ Certificate already exists."
    sudo cp /tmp/$DOMAIN.conf $REMOTE_CONF_DIR/$DOMAIN.conf
    echo "[3/3] Reloading Nginx proxy..."
    docker exec ms-nginx-proxy nginx -s reload
  else
    echo "⚠️ Certificate does not exist. Bootstrapping via Certbot..."
    
    # Create a temporary HTTP-only conf to allow certbot challenge
    cat > /tmp/${DOMAIN}_http.conf <<INNER_EOF
server {
    listen 80;
    server_name $DOMAIN;
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
}
INNER_EOF
    
    # Move temp HTTP config to proxy
    sudo cp /tmp/${DOMAIN}_http.conf $REMOTE_CONF_DIR/$DOMAIN.conf
    docker exec ms-nginx-proxy nginx -s reload
    echo "Waiting 3 seconds for Nginx to apply HTTP config..."
    sleep 3

    # Run certbot to request cert
    echo "Requesting new certificate from Let's Encrypt..."
    docker run --rm \
      -v proxy_certbot-conf:/etc/letsencrypt \
      -v proxy_certbot-www:/var/www/certbot \
      certbot/certbot certonly \
      --webroot -w /var/www/certbot \
      -d $DOMAIN \
      --email admin@openledger.vn \
      --agree-tos \
      --no-eff-email \
      --force-renewal

    # Put the real config back
    echo "Installing full HTTPS configuration..."
    sudo cp /tmp/$DOMAIN.conf $REMOTE_CONF_DIR/$DOMAIN.conf
    echo "[3/3] Reloading Nginx proxy with HTTPS..."
    docker exec ms-nginx-proxy nginx -s reload
  fi
EOF

echo "=================================================="
echo "    Gateway Deployment Completed Successfully!    "
echo "=================================================="
