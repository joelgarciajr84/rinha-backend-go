
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOFLAGS=-buildvcs=false

RUN go build -trimpath -ldflags="-s -w" -o galo .

FROM scratch

COPY --from=builder /app/galo /galo

ENV GOMEMLIMIT=64MiB \
    GOGC=100 \
    GOMAXPROCS=1

ENTRYPOINT ["/galo"]
