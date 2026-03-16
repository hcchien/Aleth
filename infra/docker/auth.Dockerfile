FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY services/auth/go.mod services/auth/go.sum ./
RUN go mod download
COPY services/auth/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth ./cmd/auth

FROM gcr.io/distroless/static-debian12
COPY --from=builder /auth /auth
EXPOSE 8081
ENTRYPOINT ["/auth"]
