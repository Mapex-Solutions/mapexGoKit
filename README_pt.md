# mapex-go-kit

> SDK Go compartilhado da plataforma
> [MapexOS](https://github.com/Mapex-Solutions/mapexOS). Todo serviço Go
> do MapexOS importa este kit para middleware HTTP, helpers NATS,
> clientes MongoDB/ClickHouse/MinIO/Redis, validação, JWT, sandbox JS,
> observabilidade e os contracts que circulam entre os serviços.

### Sobre o MapexOS

> **IoT-first, mas não se limita a IoT.**
> O MapexOS não vê dispositivos ou sensores — ele vê **Assets**.
> Qualquer fonte. Qualquer protocolo. Uma única abstração.
>
> **Connect. Automate. Scale.** — A plataforma aberta para integração de
> dados e automação inteligente.

```
   Fontes                        MapexOS                          Destinos
   ──────                        ───────                          ────────
   Devices ──┐                                              ┌── Webhooks / APIs
   Gateways ─┤   Ingest → Validate → Transform → Route →    ├── Slack / Teams / Email
   APIs ─────┼──        Store / Notify / Automate           ├── NATS / MQTT
   Apps ─────┤                                              └── Plugins customizados
   Terceiros ┘
```

Este kit fornece os blocos compartilhados de que cada caixa desse
pipeline depende.

[English version](./README.md) · [Site de documentação](https://mapexos.io)

---

## O que tem aqui

Um workspace Go com três módulos:

| Módulo | Propósito |
|---|---|
| [`infrastructure/`](./infrastructure) | Clientes e helpers tipados do data plane — MongoDB (com awareness de replica set), ClickHouse, MinIO, Redis (com distributed lock), NATS (Core + JetStream + KV + Object Store), MQTT, cliente HTTP e uma primitiva de cache em três camadas. |
| [`microservices/`](./microservices) | Plumbing transversal de serviço — servidor HTTP + middleware, logger estruturado, métricas Prometheus, validator, loader de config, container de dependências, graceful shutdown. |
| [`utils/`](./utils) | Utilitários puros — bcrypt, JWT, envelope JSON, deep flatten/copier, mapper struct ↔ DTO, runner V8 JS. |

Cada módulo tem seu próprio `go.mod` e pode ser importado
independentemente:

```go
import (
    "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb"
    "github.com/Mapex-Solutions/mapexGoKit/microservices/http"
    "github.com/Mapex-Solutions/mapexGoKit/utils/jwt"
)
```

---

## Versionamento

Este kit é a fonte da verdade para os contracts entre serviços e o
código de infraestrutura compartilhado. Mudanças que quebram são
taggeadas em `main`; serviços downstream pinam o kit via versão
taggeada no `go.mod` deles.

---

## Licença

O mapex-go-kit é distribuído sob a **Business Source License 1.1** —
veja o arquivo LICENSE no repositório
[mapexOS](https://github.com/Mapex-Solutions/mapexOS) para os termos
completos.
