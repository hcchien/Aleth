FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY services/counter/go.mod services/counter/go.sum ./
RUN go mod download
COPY services/counter/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /counter ./cmd/counter

FROM gcr.io/distroless/static-debian12
COPY --from=builder /counter /counter
ENTRYPOINT ["/counter"]
