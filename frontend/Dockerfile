FROM golang:1.25

WORKDIR /app

COPY . .

RUN go build -o frontend cmd/main.go

EXPOSE 3000

CMD ["./frontend"]
