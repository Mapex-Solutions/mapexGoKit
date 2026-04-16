module github.com/Mapex-Solutions/mapexGoKit/utils

go 1.25.3

require (
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/jinzhu/copier v0.4.0
	go.mongodb.org/mongo-driver/v2 v2.5.0
	golang.org/x/crypto v0.49.0
)

require (
	github.com/Mapex-Solutions/mapexGoKit/infrastructure v0.0.0
	github.com/Mapex-Solutions/myAIOffice/contracts v0.0.0
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/dop251/goja v0.0.0-20260311135729-065cd970411c // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/nats-io/nats.go v1.50.0 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)




replace github.com/Mapex-Solutions/mapexGoKit/infrastructure => ../infrastructure

replace github.com/Mapex-Solutions/mapexGoKit/microservices => ../microservices

replace github.com/Mapex-Solutions/myAIOffice/contracts => ../../myAIOffice/workspace_go/packages/contracts
