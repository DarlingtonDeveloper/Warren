# Stage 1: Build MC (Go binary)
FROM golang:1.22-bookworm AS go-builder

WORKDIR /build
COPY MissionControl/ ./MissionControl/

WORKDIR /build/MissionControl
RUN go build -o /mc ./cmd/mc

# Stage 2: Build mc-core (Rust binary)
FROM rust:1.75-bookworm AS rust-builder

WORKDIR /build
COPY MissionControl/core/ ./core/

WORKDIR /build/core
RUN cargo build --release && cp target/release/mc-core /mc-core

# Stage 3: Runtime — Node.js + MC binaries + supervisord
FROM node:22-bookworm AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
    supervisor \
    && rm -rf /var/lib/apt/lists/*

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Build OpenClaw gateway
WORKDIR /openclaw
COPY openclaw-src/ ./src/
WORKDIR /openclaw/src
RUN pnpm install --frozen-lockfile && pnpm build
RUN cp -r dist/openclaw.mjs /openclaw/openclaw.mjs || cp -r build/openclaw.mjs /openclaw/openclaw.mjs

# Copy MC binaries from builder stages
COPY --from=go-builder /mc /usr/local/bin/mc
COPY --from=rust-builder /mc-core /usr/local/bin/mc-core

# Copy supervisord config
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Create non-root user
RUN useradd -m -s /bin/bash dutybound
USER dutybound

EXPOSE 8081

CMD ["supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
