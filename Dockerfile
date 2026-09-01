FROM node:20-alpine AS frontend
WORKDIR /src/frontend

RUN corepack enable && corepack prepare pnpm@9 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts
COPY frontend/ ./
RUN pnpm build

FROM golang:1.26.7-alpine AS backend
WORKDIR /src

RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/gpt-mirror ./cmd/server

FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S gptmirror && adduser -S -G gptmirror gptmirror && \
    mkdir -p /app/data && chown -R gptmirror:gptmirror /app

COPY --from=backend /out/gpt-mirror /app/gpt-mirror
COPY data/config.json /app/config.default.json
COPY deploy/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod 0755 /app/gpt-mirror /app/docker-entrypoint.sh

USER gptmirror
EXPOSE 9000
VOLUME ["/app/data"]

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/gpt-mirror", "-conf", "/app/data/"]
