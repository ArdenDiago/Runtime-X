FROM golang:1.25

WORKDIR /app

# Copy entire repo (because module root is here)
COPY . .

# Build api-service binary
RUN go build -o api ./api-service/cmd/api

EXPOSE 8080

CMD ["./api"]
