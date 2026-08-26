FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY . .
# go mod tidy resolves and pins dependencies on first build, so a freshly
# cloned repo without go.sum still builds.
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 app
COPY --from=build /out/api /usr/local/bin/api
USER app
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=30s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/usr/local/bin/api"]
