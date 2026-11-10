# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/Gobierno2Go ./cmd/api

# Run stage
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/bin/Gobierno2Go /app/Gobierno2Go
COPY --from=builder /app/internal/templates /app/internal/templates

EXPOSE 8080

CMD ["/app/Gobierno2Go"]
