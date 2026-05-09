# random — Run IDs e session IDs via `crypto/rand`

Dois helpers criptograficamente seguros usados pelos serviços Mapex para identificadores efêmeros.

> Nome do pacote: `random` (diretório: `random/`).

## Superfície

```go
func NewRunID() string
func GenerateSessionID(length int) (string, error)
```

### `NewRunID`

Retorna um identificador seguro para embutir em payloads cujas entradas serão depois limpas via prefix scan. Formato:

```
YYYYMMDDhhmmss-XXXXXXXX
```

- O primeiro segmento é o timestamp em **UTC** no momento da chamada (compacto, sem separadores).
- O segundo segmento são **4 bytes aleatórios** (`crypto/rand`) hex-encoded em 8 chars.

O timestamp garante ordem lexicográfica entre runs; o sufixo aleatório garante unicidade quando vários runs começam no mesmo segundo.

```go
id := random.NewRunID() // "20260509T143012-9a7b3c4d" — amigável a ordenação + único
```

### `GenerateSessionID`

Retorna uma string hex aleatória de `2*length` caracteres (cada byte → 2 hex chars). Usa `crypto/rand`, então falha de leitura de entropia é propagada.

| `length` | Tamanho do output | Exemplo |
|---:|---:|---|
| `4` | `8` chars | `9a7b3c4d` |
| `16` | `32` chars | `4f2a9b3e8d7c6b1a…` |

```go
sid, err := random.GenerateSessionID(4)
if err != nil { return err }
```

## Notas

- `NewRunID` ignora o erro de `rand.Read` — falha de entropia produz um sufixo todo zero na prática. O uso pretendido (run IDs) não é crítico para segurança, então o trade-off é aceitável.
- `GenerateSessionID` propaga o erro para que callers possam fallback ou falhar em alto-falante.
