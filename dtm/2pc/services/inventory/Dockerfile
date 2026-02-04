FROM golang:1.24-alpine AS builder

# Install git which is required for go mod download
RUN apk --no-cache add git

WORKDIR /app

# Copy go mod files from root
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY dtm/2pc/services/inventory/*.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o service .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/service .

EXPOSE 8080

CMD ["./service"]
