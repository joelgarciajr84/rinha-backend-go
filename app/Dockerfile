
FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY ./src ./src

ENV GOFLAGS="-buildvcs=false"

RUN go build -trimpath \
    -ldflags="-s -w" \
    -o /app/transaction-processor ./src


FROM alpine:3.19


RUN apk --no-cache add ca-certificates tzdata

RUN adduser -D -g '' appuser

USER appuser
WORKDIR /app

COPY --from=builder /app/transaction-processor .

ENV GOMEMLIMIT=64MiB \
    GOGC=100 \
    GOMAXPROCS=1 \
    TZ=UTC

EXPOSE 8080

ENTRYPOINT ["./transaction-processor"]
