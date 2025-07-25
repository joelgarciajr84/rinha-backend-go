
# Rinha de Backend 2025 - Implementação em Go (Fila Assíncrona com Fallback Resiliente)


![Galo CyberPunk](/imgs/galo.png)

## Resultado dos testes locais
![Resultado Testes Locais](/imgs/teste_local.png)

Implementação competitiva e de alta performance para a **Rinha de Backend 2025**, utilizando a linguagem Go, PostgreSQL e uma arquitetura orientada a filas assíncronas com tolerância a falhas via fallback resiliente e circuit breaker.

---

## 🧱 Arquitetura: Fila Assíncrona com Fallback Resiliente

Arquitetura baseada em **event-loop com fila de tarefas em memória**, distribuída entre múltiplos workers concorrentes (modelo actor-like).

https://en.wikipedia.org/wiki/Actor_model

 Cada transação recebida é enviada para uma fila(canal) `chan`, tratada por workers independentes, que comunicam-se com serviços de processamento externos (simulados via HTTP) com fallback, healthcheck e circuit breaker.

### ✳️ Características da Arquitetura

- **Fila em memória** com até 50.000 transações concorrentes (`chan transactionTask`)
- **Pool de workers concorrentes** (40 goroutines simultâneas) (Meu magic number haha)
- **Processamento resiliente** com:
  - Health Check periódico (`GET /payments/service-health`)
  - Circuit Breaker inteligente por processador
  - Estratégia de fallback entre primário e reserva
- **Persistência confiável** em **PostgreSQL**
  - Com deduplicação (`ON CONFLICT DO NOTHING`)
  https://www.w3resource.com/PostgreSQL/snippets/postgres-on-conflict-do-nothing.php#google_vignette
  - Re-tentativas assíncronas para consistência
- **Interface REST** exposta via HTTP com handlers explícitos para transações e métricas

---

## 🎯 Justificativa da Arquitetura

A escolha pela arquitetura de **fila assíncrona com fallback resiliente** foi motivada por:

- **Baixa latência e alta vazão**: uso de filas em memória e múltiplos workers permite maximizar throughput sem bloquear o caminho crítico HTTP.
- **Simplicidade de concorrência**: goroutines e canais são leves, previsíveis e performáticos.
- **Tolerância a falhas**: componentes externos (ex: processador de pagamento) são monitorados e alternados automaticamente em caso de falha ou latência excessiva.
- **Consistência eventual com persistência**: as transações são armazenadas com segurança no banco após sucesso no processador, com re-tentativas automáticas se necessário.

---

## 🧪 Tecnologias Utilizadas

- **Go 1.23.2** – Linguagem principal, performance nativa e concorrência leve
https://go.dev/
- **Fiber** – Web framework leve baseado em `fasthttp` (não incluído diretamente aqui, mas aplicável em versões web)
https://gofiber.io/
- **PostgreSQL (via pgx/v5)** – Banco de dados relacional robusto e performático
https://github.com/jackc/pgx
- **HTTP Clients** – Comunicação com processadores externos (primário e fallback)
- **Custom Circuit Breaker** – Gerenciamento de falhas baseado em timeout e contagem
- **Health Monitor** – TTL + caching para verificação proativa dos processadores

---

## 🗃️ Organização do Código

src/
├── domain/              # DTOs, validações e estruturas de negócio
├── gateway/             # Comunicação com processadores externos (HTTP)
├── service/             # Orquestração: decide como processar e persistir
├── processor/           # Fila e workers de processamento assíncrono
├── transport/           # HTTP handlers e roteamento
├── storage/             # Acesso ao banco de dados PostgreSQL
├── circuitbreaker/      # Lógica de circuit breaker customizado
├── healthmonitor/       # Verificador de saúde dos processadores

---

## 📊 Fluxo de Processamento

1. Requisição POST chega na API (`/transactions`)
2. É validada e enviada à fila (`chan`)
3. Um dos 40 workers processa:
   - Escolhe o melhor processador (primário ou fallback)
   - Executa via HTTP
   - Se sucesso, persiste no PostgreSQL
   - Se falha, re-tenta ou descarta com log crítico
4. O GET `/transactions/summary` consulta o banco e retorna agregados por processador

---

## 🔄 Fallback e Circuit Breaker

- Cada processador (primário/fallback) possui:
  - Um circuit breaker independente
  - Health check periódico (GET health endpoint)
- Se o primário estiver indisponível, o sistema automaticamente direciona para o fallback
- Se ambos estiverem inativos, responde com erro e métricas são registradas

---

## 🧵 Concorrência

- Workers: 40
- Fila (buffer): 50.000
- Re-tentativas: configuráveis por transação
- Mutexes e sync.Map para controle de estado e cache thread-safe

---

## 🚀 Execução

```bash
docker-compose up --build
```

**Variáveis de ambiente:**

- `DB_HOST`, `DB_USER`, `DB_PASS`, `DB_NAME` – Configuração do PostgreSQL

---

## 📈 Métricas de Saúde

`GET /health` retorna:

```json
{
  "status": "healthy",
  "queue_size": 302,
  "avg_process_time": "1.7ms",
  "processed": 10200,
  "failed": 32,
  "retried": 12
}
```

---

## 📌 Considerações

- O projeto evita o uso de Redis( havia sido minha primeira opção :( ) ) ou cache externo por simplicidade operacional e menor footprint.
- O modelo de fallback e circuit breaker oferece **alta disponibilidade** mesmo com falhas parciais.
- A arquitetura é **orientada à robustez, consistência eventual(sorry ...) e desempenho extremo.**

---

Desenvolvido para a competição **Rinha de Backend 2025** com foco em **resiliência, escalabilidade e performance real-world**.
