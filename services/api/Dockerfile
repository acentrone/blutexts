FROM golang:1.22-alpine AS base
WORKDIR /app
RUN apk add --no-cache git ca-certificates

FROM base AS development
RUN go install github.com/air-verse/air@latest
COPY go.mod go.sum* ./
RUN go mod download
CMD ["air", "-c", ".air.toml"]

FROM base AS builder
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/api .

FROM alpine:3.20 AS production
RUN apk add --no-cache ca-certificates postgresql16-client
COPY --from=builder /bin/api /api
COPY --from=builder /app/migrate.sh /migrate.sh
COPY --from=builder /app/internal/db/migrations/ /app/migrations/
RUN chmod +x /migrate.sh
EXPOSE 8080 8081
ENTRYPOINT ["/migrate.sh"]
