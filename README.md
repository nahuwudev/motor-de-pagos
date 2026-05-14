# motor-de-pagos

API REST para procesar transferencias entre cuentas. Escrita en Go con `net/http` estándar, PostgreSQL y Redis.

El foco de diseño está en la **idempotencia**: garantizar que un mismo request de pago nunca genere dos cobros, sin importar cuántas veces se reintente.

---

## Stack

| Componente      | Tecnología                         |
|-----------------|------------------------------------|
| Lenguaje        | Go 1.24                            |
| HTTP            | `net/http` (sin framework)         |
| Base de datos   | PostgreSQL (pgx/v5)                |
| Lock / Caché    | Redis (go-redis/v9)                |
| Migraciones     | Goose                              |
| Queries SQL     | sqlc                               |
| Linter          | golangci-lint                      |

---

## Requisitos

- Go 1.24+
- PostgreSQL 15+
- Redis 7+
- [Goose](https://github.com/pressly/goose) (`go install github.com/pressly/goose/v3/cmd/goose@latest`)
- [sqlc](https://sqlc.dev/) (opcional, solo si modificás queries)
- [golangci-lint](https://golangci-lint.run/) (opcional, para desarrollo)

---

## Setup

### 1. Variables de entorno

Copiá `.env.example` a `.env` y completá los valores:

```env
PORT=8080
DATABASE_URL=postgres://user:password@localhost:5432/motor_de_pagos?sslmode=disable
REDIS_URL=redis://localhost:6379
```

### 2. Migraciones

```bash
make migrate-up
```

Esto aplica todas las migraciones del directorio `db/migrations/` sobre la base de datos configurada.

### 3. Compilar

```bash
make build
```

---

## Ejecución

```bash
# Modo desarrollo (sin compilar)
make run

# Binario compilado
./motor-de-pago-api
```

---

## Endpoints

### `POST /api/v1/payments`

Procesa una transferencia entre dos cuentas.

**Headers requeridos:**

```
Content-Type: application/json
Idempotency-Key: <uuid-v4-único-por-intento>
```

**Body:**

```json
{
  "from_account_id": "uuid",
  "to_account_id":   "uuid",
  "amount":          100.50,
  "currency":        "ARS"
}
```

**Respuestas:**

| Código | Descripción                                          |
|--------|------------------------------------------------------|
| `201`  | Transferencia creada                                 |
| `400`  | Parámetros inválidos                                 |
| `404`  | Cuenta no encontrada                                 |
| `409`  | Request duplicado en vuelo o con clave ya procesada  |
| `503`  | Intento previo falló, se puede reintentar            |

---

## Arquitectura: Idempotencia

### El problema

Un usuario hace clic en "Pagar" dos veces seguidas antes de que la pantalla cambie. El frontend dispara dos requests `POST /payments` idénticos con los mismos datos. Sin protección, ambos se procesarían: dos cobros, un solo pago intencionado.

Este no es un caso de borde teórico. Es el caso default en cualquier red lenta o UI sin debounce.

### La solución

El sistema usa **dos capas de defensa** que trabajan en conjunto:

#### Capa 1 — Middleware de Idempotencia (Redis)

Cada request de pago **debe** incluir un header `Idempotency-Key` con un UUID único generado por el cliente. El middleware intercepta el request antes de que llegue al handler.

El flujo para una clave nueva es:

```
Request entrante
    │
    ├─ AcquireLock (SET NX) en Redis
    │       └─ lockKey: "idem:lock:{key}"  TTL: 60s
    │
    ├─ SetPending en Redis
    │       └─ dataKey: "idem:data:{key}"  TTL: 24h  valor: "PENDING"
    │
    ├─ Ejecuta el handler real
    │
    └─ SaveResponse (Lua script atómico)
            └─ Verifica ownership del lock antes de escribir
               Escribe el payload JSON serializado en dataKey
```

Si llega un **segundo request con la misma clave** mientras el primero todavía está procesando:

```
Request duplicado
    │
    ├─ AcquireLock falla (la key ya existe en Redis)
    │
    ├─ GetResponse(dataKey)
    │       ├─ "PENDING"  → 409 Conflict ("Request already in progress")
    │       ├─ "FAILED"   → 503 + borra el lock para permitir reintento
    │       └─ JSON       → Reproduce la respuesta cacheada exacta (mismo status + body)
```

Si el request llegó **después** de que el primero terminó exitosamente, recibe la misma respuesta `201` del caché sin tocar la base de datos.

**Detalle de concurrencia:** `SaveResponse` usa un script Lua que verifica el ownership del lock antes de escribir. Si el lock expiró y otro request lo tomó, la escritura se cancela. Esto previene que dos procesos simultáneos corrompan el estado.

#### Capa 2 — Transacción ACID en PostgreSQL

Aunque el middleware de Redis detiene los duplicados en la mayoría de los casos, Redis no es la fuente de verdad. Por eso la operación central de la transferencia se ejecuta dentro de una **transacción PostgreSQL** con bloqueo explícito a nivel de fila.

```sql
-- Bloqueamos las filas de ambas cuentas antes de leer saldos
SELECT ... FROM payments.accounts WHERE id = $1 FOR NO KEY UPDATE;
```

Dentro de la misma transacción:
1. Se verifica que la cuenta origen tenga fondos suficientes
2. Se actualiza el saldo de ambas cuentas (`balance + delta`)
3. Se inserta el registro de la transacción

Si cualquiera de estos pasos falla, **toda la transacción hace rollback**. No hay estados intermedios posibles.

La combinación de ambas capas significa:
- Redis detiene el duplicado en el borde de la red, en milisegundos, sin costo para la DB.
- PostgreSQL garantiza consistencia incluso si Redis falla o expira el lock antes de tiempo.

### Estructura del proyecto

```
.
├── cmd/
│   └── main.go                  # Punto de entrada, wiring de dependencias
├── db/
│   ├── migrations/              # Migraciones SQL (Goose)
│   └── queries/                 # Queries SQL (sqlc)
├── internal/
│   ├── api/
│   │   ├── handler/
│   │   │   └── payment.go       # Handler HTTP, validación del request
│   │   └── middleware/
│   │       └── idempotency.go   # Middleware de idempotencia
│   ├── config/                  # Carga de variables de entorno
│   ├── db/                      # Pools de conexión (pgx, redis)
│   ├── idempotency/             # Interfaz + implementación Redis del store
│   └── payments/                # Lógica de negocio + repositorio SQL
└── makefile
```

---

## Desarrollo

```bash
# Lint
make lint

# Migración hacia atrás
make migrate-down

# Regenerar código desde queries SQL (requiere sqlc)
sqlc generate
```
