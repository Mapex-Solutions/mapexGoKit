# time — Helpers de RFC3339-com-milissegundos (`timeUtil`)

Helpers pequenos sobre `time.Time` para o layout ISO style-MongoDB (`RFC3339` + `.000` milissegundos). Úteis para DTOs JSON que precisam serializar valores zero como `null` e valores não-zero com precisão de milissegundo.

> Nome do pacote: `timeUtil` (diretório: `time/`).

## Superfície

```go
const RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

func ToRFC3339Milli(t time.Time) *string

type NullTime struct { time.Time }
func (NullTime) MarshalJSON() ([]byte, error)
```

### `ToRFC3339Milli`

Retorna um `*string`:

- `nil` quando `t.IsZero()`.
- Caso contrário, o instante em UTC formatado com `RFC3339Milli`.

Útil para campos opcionais em DTOs onde omitir/`null` é mais correto que `"0001-01-01T..."`.

### `NullTime`

Wrapper de `time.Time` com `MarshalJSON` que emite:

- `null` quando o time interno é zero.
- String RFC3339-milissegundo em UTC entre aspas caso contrário.

Embeda em DTOs:

```go
type AssetDTO struct {
    Created *timeUtil.NullTime `json:"created"`
    Updated *timeUtil.NullTime `json:"updated"`
}
```

`NullTime` apenas customiza a serialização. **Não implementa `UnmarshalJSON`**, então `null` no JSON não sobrevive ao round-trip — o campo continua no valor zero, o que normalmente é o desejado para estes campos convencionais.
