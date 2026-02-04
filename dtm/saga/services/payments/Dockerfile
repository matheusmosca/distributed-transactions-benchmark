FROM golang:1.24.1-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum from project root
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy service source code (mantém estrutura de diretórios)
COPY dtm/ ./dtm/

# Build the service (compila todo o pacote)
RUN CGO_ENABLED=0 GOOS=linux go build -o payments-service ./dtm/saga/services/payments

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/payments-service .

EXPOSE 8080

CMD ["./payments-service"]
