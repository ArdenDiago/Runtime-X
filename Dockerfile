# Stage 1: Build the React frontend
FROM node:22-slim AS frontend
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build the Go binary
FROM golang:tip-trixie AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o rtx ./cmd/rtx

# Stage 3: Runtime image
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    procps \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the compiled binary
COPY --from=backend /app/rtx ./rtx

# Copy the built frontend assets
COPY --from=frontend /web/dist ./web/dist

EXPOSE 8080

# Default to serve mode — override with: docker run rtx run <command>
ENTRYPOINT ["./rtx"]
CMD ["serve"]
