// Package natsjwt issues and parses NATS user JWTs (the Ed25519/nkey-signed
// credentials NATS leaf nodes validate locally during decentralized auth).
//
// The package wraps github.com/nats-io/jwt/v2 with a flat API mirroring
// the shape of the project's other small utility packages
// (utils/zerovalue, utils/random). Consumers typically use IssueUserJWT
// during device provisioning and ParseUserJWT in HTTP middlewares that
// verify a device-presented bearer token.
package natsjwt

// UserClaims is the subset of NATS user-JWT claims this codebase cares
// about. Name carries the asset uuid (NATS' "name" claim, surfaced in
// $SYS.ACCOUNT.*.CONNECT/DISCONNECT advisories so the presence consumer
// can identify the device). Tags is a flat string map used for
// orgId / assetUUID / templateId metadata. Pub and Sub list the
// subjects the device may publish or subscribe on; the leaf rejects
// any operation outside these allow/deny lists. Issuer is the public
// key that signed the JWT — middlewares performing trust-anchor checks
// compare this against the platform signing key's public.
//
// BearerToken switches the user JWT to "bearer" mode: the leaf accepts
// the JWT without challenging the device to sign a connection nonce
// with the user nkey seed. This is the IoT-friendly path — devices
// connect with Username = devID + Password = JWT and never need to
// hold the user nkey seed. Set true for MQTT-protocol device JWTs.
type UserClaims struct {
	Name        string
	Issuer      string
	BearerToken bool
	Tags        map[string]string
	Pub         PermissionList
	Sub         PermissionList
}

// PermissionList captures the allow/deny pair NATS uses for both pub
// and sub claims. An empty Allow with empty Deny means "no permission"
// (the broker rejects everything); a non-empty Allow with empty Deny
// means "exactly these subjects are allowed".
type PermissionList struct {
	Allow []string
	Deny  []string
}
