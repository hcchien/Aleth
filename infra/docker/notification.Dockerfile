FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY services/notification/go.mod services/notification/go.sum ./
RUN go mod download
COPY services/notification/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /notification ./cmd/notification

FROM gcr.io/distroless/static-debian12
COPY --from=builder /notification /notification
EXPOSE 8086
ENTRYPOINT ["/notification"]
