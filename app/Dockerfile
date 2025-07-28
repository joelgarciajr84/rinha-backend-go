FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY ./ /app
ENV GOFLAGS="-buildvcs=false"

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags="-s -w" \
    -o galo .
RUN apk add --no-cache file
RUN file galo

FROM alpine:latest
COPY --from=builder /app/galo /usr/local/bin/
RUN chmod +x /usr/local/bin/galo
ENV GOMEMLIMIT=64MiB \
    GOGC=100 \
    GOMAXPROCS=1
CMD ["galo"]
