FROM golang:latest AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]