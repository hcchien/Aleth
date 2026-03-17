FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY services/gateway/go.mod services/gateway/go.sum ./
RUN go mod download
COPY services/gateway/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12
COPY --from=builder /gateway /gateway
EXPOSE 8080
ENTRYPOINT ["/gateway"]
