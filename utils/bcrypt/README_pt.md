# bcrypt — Helpers de hash de senha

Wrapper de duas linhas sobre `golang.org/x/crypto/bcrypt` que expõe apenas o que serviços precisam: gerar hash de senha em texto e verificar par hash/texto. Vive em `bcrypt/password/` (pacote `password`).

> Nome do pacote: `password` (diretório: `bcrypt/password/`).

## Superfície

```go
func HashPassword(password string) (string, error)
func CheckPassword(hashedPassword string, plainPassword string) bool
```

| Função | Comportamento |
|---|---|
| `HashPassword(plain)` | Chama `bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)` e retorna o hash como string. Retorna `("", err)` em falha (tipicamente senha muito longa — bcrypt corta em 72 bytes). |
| `CheckPassword(hash, plain)` | Retorna `true` apenas se `bcrypt.CompareHashAndPassword` tem sucesso. **Qualquer erro**, inclusive hash malformado, vira `false`. |

O custo é hardcoded em `bcrypt.DefaultCost` (10 no momento da escrita) — equilíbrio sensato para a maioria dos serviços; aumente no código se precisar de fator de trabalho mais forte.

## Uso

```go
hash, err := password.HashPassword(req.Password)
if err != nil { return err }

// depois, no login
if !password.CheckPassword(stored.PasswordHash, req.Password) {
    return ErrInvalidCredentials
}
```

## Notas

- `CheckPassword` colapsa todo erro em `false`, então logue o diagnóstico por conta própria se precisar diferenciar "senha errada" de "hash corrompido".
- bcrypt trunca senhas com mais de 72 bytes — `HashPassword` retorna erro nesse caso ao invés de truncar silenciosamente.
