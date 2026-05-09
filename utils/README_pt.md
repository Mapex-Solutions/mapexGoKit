# utils — Utilitários compartilhados Mapex

Vinte pacotes pequenos e autocontidos usados por `infrastructure/` e `microservices/`. Cada um tem um propósito único e bem delimitado — nenhum pacote aqui conhece HTTP, NATS, MongoDB, etc. exceto se o pacote em si é *sobre* aquela preocupação (ex: `natsjwt`, `orgfilter`).

## Subpacotes

| Pacote | O que é |
|---|---|
| [`bcrypt/password`](./bcrypt) | Helpers bcrypt de hash + verificação em duas linhas (default cost). |
| [`deepcopy`](./deepcopy) | Deep copy via JSON-roundtrip para `map[string]any` (e map-de-maps). |
| [`envelope`](./envelope) | Envelope encryption AES-256-GCM (Master Key + DEK por registro). |
| [`flatten`](./flatten) | Tradução map aninhado ↔ chaves planas + lookup por path com fan-out de array. |
| [`jsrunner`](./jsrunner) | Runner ES5 sandboxed sobre `goja` com timeout. |
| [`jwt`](./jwt) | Helpers HS256 de access + refresh token (`golang-jwt/jwt/v5`). |
| [`mapper`](./mapper) | Cópia genérica DTO ↔ Entity com conversões de ObjectId e taps de Created/Updated. |
| [`natsjwt`](./natsjwt) | Emite e parseia NATS user JWTs (Ed25519/nkey, modo bearer). |
| [`orgfilter`](./orgfilter) | Filtros multi-tenant de org (Mongo + ClickHouse), escopo por pathKey, visibilidade de templates por ancestrais. |
| [`pathkey`](./pathkey) | Utilitários de pathKey hierárquico — next-sibling, ancestrais, checks de descendente. |
| [`random`](./random) | Run IDs e session IDs com base em `crypto/rand`. |
| [`serialize`](./serialize) | Wrappers finos de marshal/unmarshal JSON para troca de impl. |
| [`slice`](./slice) | Utilitários genéricos de slice (hoje só `Reverse`). |
| [`templatereplace`](./templatereplace) | Interpolação `{{path.to.value}}` em árvores JSON-like. |
| [`time`](./time) | Helpers RFC3339-com-milissegundos + `NullTime`. |
| [`typeconv`](./typeconv) | Conversões permissivas `interface{}` → tipo com variantes `Tryxxx`. |
| [`validations`](./validations) | Singleton de `go-playground/validator` com extras Mapex (`mongoid`, `uuid`) e mensagens humanizadas. |
| [`zerovalue`](./zerovalue) | Geradores de valor zero por chave string mais `Ptr[T]` genérico. |

## Como os pacotes se relacionam

A maioria é independente. Os edges cross-package que importam:

- `mapper` depende de `serialize` (JSON round-trips) e `time` (`NullTime`).
- `orgfilter` depende de `pathkey` (queries de range) e de `microservices/common/context` + `infrastructure/mongodb/model` (escopo de request e parse de ObjectID).
- `validations/customvalidation` é o lar para novas regras field-level; `validations.New()` as registra no primeiro uso.
- `serialize` é o ponto único de entrada para JSON — altere `serialize.Marshal/Unmarshal` para trocar o encoder globalmente.

## Convenções

- **Singletons** são protegidos por `sync.Once` (`validations.New`, `microservices/logger.InitLogger`, etc.) — primeira chamada vence, chamadas seguintes retornam a instância existente.
- **Sem erros HTTP aqui.** Funções retornam `error` ou sentinel `errors.New(...)` — consumidores mapeiam para status codes.
- **Erros que significam "nada aconteceu"** (ex: `pathkey.IsDescendant` retornando `false`) são retornados como `bool`, não `error`.
- **Deep copies são em formato JSON.** `deepcopy` e `mapper.*Map*` fazem round-trip via JSON — valores não-JSON (channels, funções, NaN/Inf, precisão de time) perdem informação.

Para detalhes de cada subpacote — tipos, defaults, edge cases, comportamentos testados — veja o README em cada diretório.
