# Multi-stage Docker build for tvcalendar
# Stage 1: Build the binary
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tvcalendar ./cmd/tvcalendar
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tv-notifier ./cmd/tv-notifier

# Stage 2: Minimal distroless scratch image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /tvcalendar /usr/local/bin/tvcalendar
COPY --from=builder /tv-notifier /usr/local/bin/tv-notifier

EXPOSE 8080

ENTRYPOINT ["tvcalendar"]
CMD ["serve", "--port", "8080"]
