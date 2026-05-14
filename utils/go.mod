module github.com/Mapex-Solutions/mapexGoKit/utils

go 1.25.3

require (
	github.com/go-playground/validator/v10 v10.27.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/jinzhu/copier v0.4.0
	github.com/nats-io/jwt/v2 v2.8.1
	github.com/nats-io/nkeys v0.4.15
	go.mongodb.org/mongo-driver/v2 v2.5.0
	golang.org/x/crypto v0.49.0
)

require (
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
)

require (
	github.com/Mapex-Solutions/mapexGoKit/infrastructure v0.0.0
	github.com/Mapex-Solutions/mapexGoKit/microservices v0.0.0
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/dop251/goja v0.0.0-20260311135729-065cd970411c
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace github.com/Mapex-Solutions/mapexGoKit/infrastructure => ../infrastructure

replace github.com/Mapex-Solutions/mapexGoKit/microservices => ../microservices
