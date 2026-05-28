# mapex-go-kit

> Shared Go SDK for the [MapexOS](https://github.com/Mapex-Solutions/mapexOS)
> platform. Every Go service in MapexOS imports this kit for HTTP
> middleware, NATS helpers, MongoDB/ClickHouse/MinIO/Redis clients,
> validation, JWT, JS sandboxing, observability, and the contracts
> that flow between services.

### About MapexOS

> **IoT-first, but not limited to IoT.**
> MapexOS doesn't see devices or sensors — it sees **Assets**.
> Any source. Any protocol. One abstraction.
>
> **Connect. Automate. Scale.** — The open platform for data integration
> and intelligent automation.

```
   Sources                       MapexOS                         Destinations
   ───────                       ───────                         ────────────
   Devices ──┐                                              ┌── Webhooks / APIs
   Gateways ─┤   Ingest → Validate → Transform → Route →    ├── Slack / Teams / Email
   APIs ─────┼──        Store / Notify / Automate           ├── NATS / MQTT
   Apps ─────┤                                              └── Custom plugins
   3rd-party ┘
```

This kit provides the shared building blocks that every box in that
pipeline depends on.

[Versão em português](./README_pt.md) · [Documentation site](https://mapexos.io)

---

## What's inside

A Go workspace with three modules:

| Module | Purpose |
|---|---|
| [`infrastructure/`](./infrastructure) | Typed clients and helpers for the platform's data plane — MongoDB (replica-aware), ClickHouse, MinIO, Redis (with distributed lock), NATS (Core + JetStream + KV + Object Store), MQTT, HTTP client, and a three-tier cache primitive. |
| [`microservices/`](./microservices) | Cross-cutting service plumbing — HTTP server + middleware, structured logger, Prometheus metrics, validator, config loader, dependency container, graceful shutdown. |
| [`utils/`](./utils) | Pure utilities — bcrypt, JWT, JSON envelope, deep flatten/copier, struct ↔ DTO mapper, V8 JS runner. |

Each module has its own `go.mod` and can be imported independently:

```go
import (
    "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb"
    "github.com/Mapex-Solutions/mapexGoKit/microservices/http"
    "github.com/Mapex-Solutions/mapexGoKit/utils/jwt"
)
```

---

## Versioning

This kit is the source of truth for cross-service contracts and shared
infrastructure code. Breaking changes are tagged on `main`; downstream
services pin the kit via a tagged version in their `go.mod`.

---

## License

mapex-go-kit is distributed under the **Business Source License 1.1** —
see the LICENSE file in the [mapexOS](https://github.com/Mapex-Solutions/mapexOS)
repository for the full terms.
