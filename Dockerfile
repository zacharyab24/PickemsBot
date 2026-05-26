# Stage 1: build the binary
FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o pickems .

# Stage 2: runtime image with Chromium for the bracket renderer
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    ca-certificates \
    fonts-liberation \
    && ln -s /usr/bin/chromium /usr/bin/chromium-browser \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/pickems .
COPY resources/ resources/

# Required environment variables at runtime:
#   DISCORD_PROD_TOKEN    - Discord bot token
#   DISCORD_BETA_TOKEN    - Discord bot token (test server)
#   MONGO_PROD_URI        - MongoDB connection string
#   LIQUIDPEDIADB_API_KEY - Liquipedia API key (liquipedia data source only)
#   PANDASCORE_API_KEY    - PandaScore API key (pandascore data source only)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:9090/health || exit 1

CMD ["./pickems"]
