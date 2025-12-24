#!/bin/bash

if [ -z "$1" ]; then
  echo "SVPS Network Tunneling Utility"
  echo "Usage: expose <port>"
  echo "Example: expose 25565"
  exit 1
fi

PORT=$1

echo "[SVPS-NET] Initializing secure tunnel for localhost:$PORT..."
echo "[SVPS-NET] Negotiating with edge network..."

# Menggunakan Cloudflared Quick Tunnel
# Opsi --url tcp:// mengekspos raw TCP stream
cloudflared tunnel --url tcp://localhost:$PORT
