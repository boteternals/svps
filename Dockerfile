# --- STAGE 1: BUILDER ---
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
# Build Clean ETP Engine
RUN rm -f go.sum && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /svps-server core/main.go

# --- STAGE 2: RUNTIME (TITAN) ---
FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive

# 1. System Dependencies (Podman & Utilities)
RUN apt-get update && apt-get install -y \
    curl git vim htop wget sudo net-tools iputils-ping \
    python3 python3-pip nano tmux screen \
    nginx php-fpm mariadb-server \
    podman fuse-overlayfs uidmap \
    iptables xz-utils \
    && rm -rf /var/lib/apt/lists/*

# 2. Cloudflared Installation (Network Tunnel)
RUN wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb \
    && dpkg -i cloudflared-linux-amd64.deb \
    && rm cloudflared-linux-amd64.deb

# 3. S6-Overlay Installation (Init System)
ARG S6_VER=3.1.6.2
RUN curl -L -o /tmp/s6-overlay-noarch.tar.xz https://github.com/just-containers/s6-overlay/releases/download/v${S6_VER}/s6-overlay-noarch.tar.xz && \
    tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && \
    curl -L -o /tmp/s6-overlay-x86_64.tar.xz https://github.com/just-containers/s6-overlay/releases/download/v${S6_VER}/s6-overlay-x86_64.tar.xz && \
    tar -C / -Jxpf /tmp/s6-overlay-x86_64.tar.xz && \
    rm /tmp/*.tar.xz

# 4. Podman Configuration (FIXED SECTION)
# Kita buat config manual agar driver VFS aktif (Stable di PaaS)
RUN mkdir -p /etc/containers \
    && echo '[storage]' > /etc/containers/storage.conf \
    && echo 'driver = "vfs"' >> /etc/containers/storage.conf \
    && echo 'runroot = "/run/containers/storage"' >> /etc/containers/storage.conf \
    && echo 'graphroot = "/var/lib/containers/storage"' >> /etc/containers/storage.conf \
    # Create Alias for Emulation
    && echo 'alias docker=podman' >> /root/.bashrc \
    && echo 'alias docker-compose=podman-compose' >> /root/.bashrc

# 5. Asset Placement
# Copy Engine
COPY --from=builder /svps-server /usr/local/bin/svps-server

# Copy Scripts (Emulation Layer)
COPY scripts/systemctl_shim.py /usr/bin/systemctl
COPY scripts/tunnel_manager.sh /usr/bin/expose
RUN chmod +x /usr/bin/systemctl /usr/bin/expose

# Configure S6 Service for SVPS
RUN mkdir -p /etc/s6-overlay/s6-rc.d/svps-engine
COPY init/svps_service /etc/s6-overlay/s6-rc.d/svps-engine/run
RUN echo "longrun" > /etc/s6-overlay/s6-rc.d/svps-engine/type \
    && touch /etc/s6-overlay/s6-rc.d/user/contents.d/svps-engine \
    && chmod +x /etc/s6-overlay/s6-rc.d/svps-engine/run

# 6. Environment & Entry
ENV TERM=xterm-256color
ENV SHELL=/bin/bash
WORKDIR /root
EXPOSE 8080

# Handover control to S6 Init System
ENTRYPOINT ["/init"]

