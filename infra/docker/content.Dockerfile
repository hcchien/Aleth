FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY services/content/go.mod services/content/go.sum ./
RUN go mod download
COPY services/content/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /content ./cmd/content

FROM gcr.io/distroless/static-debian12
COPY --from=builder /content /content
EXPOSE 8082
ENTRYPOINT ["/content"]
