FROM node:24-alpine AS spa
WORKDIR /app/web
RUN corepack enable
# pnpm version is pinned via the "packageManager" field in package.json.
# .npmrc (the @telliott scope, read anonymously) and pnpm-workspace.yaml (pnpm's
# build-script approvals) must land here, not in the `COPY web/ ./` below.
COPY web/package.json web/pnpm-lock.yaml web/.npmrc web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=spa /app/web/static ./web/static
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o /out/damson ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S damson \
    && adduser -S damson -G damson
USER damson
COPY --from=build /out/damson /usr/local/bin/damson
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -q -O- http://localhost:8080/api/health || exit 1
ENTRYPOINT ["/usr/local/bin/damson"]
