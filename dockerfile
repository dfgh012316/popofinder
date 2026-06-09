# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o popofinder ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1 \
    && find /go/bin -name migrate -type f | head -1 | xargs -I{} cp {} /build/migrate

# Stage 2: Runtime (distroless static — no shell, minimal attack surface)
FROM gcr.io/distroless/static-debian12

COPY --from=builder /build/popofinder /popofinder
COPY --from=builder /build/migrate /migrate
COPY migrations /migrations

USER 1000
EXPOSE 8000
ENTRYPOINT ["/popofinder"]
