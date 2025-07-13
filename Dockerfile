FROM golang:1.24 AS builder

WORKDIR /app

COPY app/ .

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN go build -a -installsuffix cgo -o main ./cmd

FROM alpine:3.20

WORKDIR /root/

COPY --from=builder /app/main .

CMD ["./main"]
