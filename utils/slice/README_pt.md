# slice — Utilitários genéricos para slice

Hoje: um único helper. O pacote é o lar de qualquer operação genérica futura sobre slices.

> Nome do pacote: `slice` (diretório: `slice/`).

## Superfície

```go
func Reverse[T any](s []T)
```

Reversa in-place — troca `s[i]` com `s[len(s)-1-i]` até se encontrarem. Muta o slice do caller.

## Uso

```go
xs := []int{1, 2, 3, 4, 5}
slice.Reverse(xs)
// xs == [5 4 3 2 1]
```
