FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY services/feed/go.mod services/feed/go.sum ./
RUN go mod download
COPY services/feed/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /feed ./cmd/feed

FROM gcr.io/distroless/static-debian12
COPY --from=builder /feed /feed
EXPOSE 8083
ENTRYPOINT ["/feed"]
