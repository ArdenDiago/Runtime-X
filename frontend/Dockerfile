FROM golang:1.25 AS builder

WORKDIR /app

# Copy only necessary files
COPY cmd/ ./cmd/
COPY templates/ ./templates/

# Build the Go frontend binary
RUN go build -o frontend ./cmd/main.go

FROM golang:1.25
WORKDIR /app

# Copy the built binary and templates from builder
COPY --from=builder /app/frontend ./frontend
COPY --from=builder /app/templates ./templates

EXPOSE 3000

CMD ["./frontend"]
