FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY services/federation/go.mod services/federation/go.sum ./
RUN go mod download
COPY services/federation/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /federation ./cmd/federation

FROM gcr.io/distroless/static-debian12
COPY --from=builder /federation /federation
EXPOSE 8084
ENTRYPOINT ["/federation"]
