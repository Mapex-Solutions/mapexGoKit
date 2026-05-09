# bcrypt — Password hashing helpers

Two-line wrapper around `golang.org/x/crypto/bcrypt` that exposes only what services need: hash a plain password and verify a hashed/plain pair. Lives under `bcrypt/password/` (the `password` package).

> Package name: `password` (directory: `bcrypt/password/`).

## Surface

```go
func HashPassword(password string) (string, error)
func CheckPassword(hashedPassword string, plainPassword string) bool
```

| Function | Behaviour |
|---|---|
| `HashPassword(plain)` | Calls `bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)` and returns the hash as a string. Returns `("", err)` on failure (typically password too long — bcrypt caps at 72 bytes). |
| `CheckPassword(hash, plain)` | Returns `true` iff `bcrypt.CompareHashAndPassword` succeeds. **Any error**, including malformed-hash, is collapsed to `false`. |

The cost is hard-coded to `bcrypt.DefaultCost` (10 at the time of writing) — a sensible balance for most services; bump in code if you need stronger work factor.

## Usage

```go
hash, err := password.HashPassword(req.Password)
if err != nil { return err }

// later, on login
if !password.CheckPassword(stored.PasswordHash, req.Password) {
    return ErrInvalidCredentials
}
```

## Notes

- `CheckPassword` collapses every error to `false`, so log diagnostics yourself if you need to differentiate "wrong password" from "corrupt hash".
- bcrypt truncates passwords longer than 72 bytes — `HashPassword` will return an error in that case rather than silently truncating.
