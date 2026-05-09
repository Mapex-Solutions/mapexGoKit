# typeconv — Conversões permissivas `interface{}` → tipo

Helpers em runtime para formatos que chegam como `interface{}` (JSON parseado, documentos MongoDB, query params, schemas dinâmicos, etc.). Toda conversão tem duas formas — `Toxxx(value) (xxx, error)` e `Tryxxx(value) (xxx, bool)` — para encaixar em call sites com erro ou em estilo booleano.

> Nome do pacote: `typeconv` (diretório: `typeconv/`).

## Erros

```go
var ErrNilValue       = fmt.Errorf("nil value")
var ErrTypeConversion = fmt.Errorf("type conversion error")
```

`Toxxx` embrulha a falha subjacente com `%w: …`, então `errors.Is(err, ErrTypeConversion)` funciona.

## Conversões numéricas

```go
func ToFloat64(v interface{}) (float64, error)
func TryFloat64(v interface{}) (float64, bool)
func ToInt64(v interface{})   (int64, error)
func TryInt64(v interface{})  (int64, bool)
```

Inputs aceitos:

| Origem | Comportamento |
|---|---|
| `float64`/`float32` | Direto |
| `int`/`int8…64` / `uint`/`uint8…64` | Cast |
| `bool` | `true→1`, `false→0` |
| `string` | `strconv.ParseFloat` (apenas strings numéricas) |
| qualquer outro | `ErrTypeConversion: unsupported type %T for number conversion` |
| `nil` | `ErrNilValue` |

`ToInt64` trunca o resultado de `ToFloat64`.

## Conversão para string

```go
func ToString(v interface{}) (string, error)
func TryString(v interface{}) (string, bool)
```

| Origem | Comportamento |
|---|---|
| `string` | Direto |
| `[]byte` | `string(v)` |
| `fmt.Stringer` | `v.String()` |
| `bool` | `strconv.FormatBool` |
| inteiros / floats | `fmt.Sprintf("%d"/"%v", ...)` |
| qualquer outro | `fmt.Sprintf("%v", v)` (nunca erra) |
| `nil` | `ErrNilValue` |

## Conversão para bool

```go
func ToBool(v interface{}) (bool, error)
func TryBool(v interface{}) (bool, bool)
```

Strings aceitas (case-insensitive, trimadas): `"true"`, `"1"`, `"yes"`, `"on"` → `true`; `"false"`, `"0"`, `"no"`, `"off"`, `""` → `false`. Qualquer outra string gera `ErrTypeConversion`. Inputs numéricos seguem "diferente de zero é true".

## Conversão para time

```go
func ToTime(v interface{}, timezone string) (time.Time, error)
func TryTime(v interface{}, timezone string) (time.Time, bool)
```

`timezone` é o nome passado para `time.LoadLocation`. Vazio ou inválido → cai em UTC (`GetLocation`).

| Origem | Tratada como |
|---|---|
| `time.Time` | Relocalizada via `t.In(loc)` |
| `string` | Tentada contra `DateFormats` e depois `TimeFormats` (ver abaixo); primeiro match vence |
| `int64` / `int` | Unix seconds (`time.Unix(v, 0)`) |
| `float64` | Unix seconds + fração sub-segundo (`(v - sec) * 1e9` ns) |
| qualquer outro | `ErrTypeConversion` |
| `nil` | `ErrNilValue` |

### `DateFormats`

```go
[]string{
    time.RFC3339,
    time.RFC3339Nano,
    "2006-01-02T15:04:05",
    "2006-01-02 15:04:05",
    "2006-01-02",
    "02/01/2006",
    "01/02/2006",
    "2006/01/02",
}
```

### `TimeFormats`

```go
[]string{"15:04:05", "15:04", "3:04 PM", "3:04:05 PM"}
```

Ambas as listas são `var`s no nível do pacote — atribua antes de parsear se precisar adicionar formatos globalmente.

## Helpers de timezone

```go
func GetLocation(timezone string) *time.Location  // vazio/inválido → UTC
func GetCurrentTime(timezone string) time.Time    // = time.Now().In(GetLocation(tz))
```

## Checks de tipo

```go
func IsNumeric(v interface{}) bool
func IsString(v interface{}) bool
func IsBool(v interface{}) bool
```

`IsNumeric` e `IsBool` usam o `Tryxxx` correspondente (então uma string numérica retorna `true` em `IsNumeric`). `IsString` é assert estrito — apenas `string` retorna `true`.

## Factories de ponteiro

```go
func PtrString(s string)    *string
func PtrBool(b bool)        *bool
func PtrInt(i int)          *int
func PtrInt64(i int64)      *int64
func PtrFloat64(f float64)  *float64
func PtrTime(t time.Time)   *time.Time
```

Criadores inline de ponteiro para fields opcionais de DTO/query. Espelham `zerovalue.Ptr[T]` para os tipos escalares comuns.

## Conversão dinâmica

```go
func ConvertByKind(value interface{}, kind string, timezone string) (interface{}, error)
```

Roteia por `kind` em lowercase:

| `kind` | Mapeia para |
|---|---|
| `"string"` | `ToString` |
| `"number"` / `"float"` / `"integer"` / `"int"` | `ToFloat64` |
| `"boolean"` / `"bool"` | `ToBool` |
| `"date"` / `"datetime"` / `"time"` | `ToTime(value, timezone)` |
| qualquer outro | Retorna o valor como está, sem erro |
| `value` nil | `ErrNilValue` |

Útil para schemas dinâmicos onde o tipo do campo é descrito como string.

## Notas

- `ToFloat64` / `ToInt64` aceitam booleanos (intencional). Se precisa de input estritamente numérico, faça type-assert antes.
- O caminho `"int"`/`"integer"` de `ConvertByKind` retorna um `float64`, não um `int64` — atenção quando o downstream espera inteiro (chame `ToInt64` ao invés).
- `ToString` nunca retorna `ErrTypeConversion` — todo valor não-nil cai no `%v`. O caminho de erro só dispara em `nil`.
