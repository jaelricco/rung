FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
# Dependencies first, as their own layer: go.mod and go.sum change rarely, so
# the module download is reused across builds where only source changed. This
# is most of the CI build time.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# No `go mod tidy` here. go.sum is committed and CI verifies it, so the build
# resolves nothing at image-build time and two builds of one commit produce the
# same binary.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 app
COPY --from=build /out/api /usr/local/bin/api
USER app
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=30s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/usr/local/bin/api"]
