FROM golang:latest AS builder

WORKDIR /app

COPY app/ .

# Define variáveis para build estático
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# Compila o binário a partir do main.go localizado em cmd/
RUN go build -a -installsuffix cgo -o main ./cmd

# Etapa 2: Imagem final enxuta
FROM alpine:latest

WORKDIR /root/

# Copia apenas o binário
COPY --from=builder /app/main .

# Executa o binário
CMD ["./main"]
