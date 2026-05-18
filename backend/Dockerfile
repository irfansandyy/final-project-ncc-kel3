FROM golang:1.24-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl \
    && addgroup -S app \
    && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/server /app/server

ENV APP_PORT=80
EXPOSE 80

HEALTHCHECK CMD curl --fail http://localhost/health || exit 1

USER app
ENTRYPOINT ["/app/server"]
