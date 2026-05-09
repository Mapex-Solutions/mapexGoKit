# templatereplace — Interpolação `{{path.to.value}}` em árvores JSON-like

Substitui recursivamente placeholders `{{path.to.value}}` dentro de qualquer valor (string, `map[string]any`, `[]any`) usando um `contexts map[string]any` plano cujos valores podem ser maps aninhados. Função pura, sem dependências além da stdlib.

> Nome do pacote: `templatereplace` (diretório: `templatereplace/`).

## Superfície

```go
func Resolve(value interface{}, contexts map[string]interface{}) interface{}
func ResolveString(s string, contexts map[string]interface{}) string
```

| Função | Comportamento |
|---|---|
| `Resolve(value, contexts)` | Percorre `string` / `map[string]interface{}` / `[]interface{}` recursivamente. Outros tipos saem como estão (sem cópia). Novos maps/slices são alocados; o input não é mutado. |
| `ResolveString(s, contexts)` | Substitui todas as ocorrências `{{…}}` em uma única string. Faz short-circuit quando a string não contém `"{{"`. |

## Algoritmo de resolução

1. Casa todo grupo `{{...}}` via regex no nível do pacote.
2. Remove as chaves → o miolo é tratado como path por pontos (ex: `config.chatId`).
3. Percorre `contexts` segmento por segmento: cada um precisa resolver para um valor (terminal) ou um `map[string]interface{}` (intermediário).
4. Se o path resolve, substitui o match por `fmt.Sprintf("%v", resolved)`.
5. Se qualquer segmento estiver ausente ou um intermediário não for map, **deixa o texto `{{...}}` original intacto** — útil para templates que intencionalmente passam por múltiplos passos de resolução (ex: um placeholder `{{before.token}}` que será preenchido depois).

## Uso

```go
contexts := map[string]interface{}{
    "config":   map[string]interface{}{"chatId": "abc-123"},
    "manifest": map[string]interface{}{"defaults": map[string]interface{}{"baseUrl": "https://api"}},
}

// String única
templatereplace.ResolveString("{{config.chatId}}", contexts) // "abc-123"

// Estrutura aninhada
payload := map[string]interface{}{
    "url":  "{{manifest.defaults.baseUrl}}/v1",
    "tags": []interface{}{"chat:{{config.chatId}}", "{{unresolved.path}}"},
}
out := templatereplace.Resolve(payload, contexts)
// out["url"]  == "https://api/v1"
// out["tags"] == ["chat:abc-123", "{{unresolved.path}}"]   // 2º ficou intacto
```

## Notas

- O regex de placeholder é `\{\{([^}]+)\}\}` — **não** permite `}` dentro do path. Útil na prática; não passe paths contendo `}`.
- A resolução sempre converte o valor via `fmt.Sprintf("%v", v)`. Valores de tipos complexos (maps, slices) viram a representação default do Go — normalmente não é o que você quer; mantenha placeholders terminais apontando para escalares.
