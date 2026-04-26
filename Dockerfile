FROM node:24-alpine AS spa
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

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
