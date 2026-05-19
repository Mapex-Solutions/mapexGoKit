// Package configuration — production-default guard.
//
// Many services declare dev-friendly Default values for credentials and
// secrets so they boot out of the box in local development. Those defaults
// must never reach production. This file provides the detection logic used
// by InitConfig to refuse startup when a sensitive key is still using its
// hardcoded Default in a non-dev environment, and to emit a visible
// warning when the same condition is detected in local dev.
package configuration

import "reflect"

// Violation identifies a Sensitive configuration key whose resolved value
// still equals its hardcoded Default.
type Violation struct {
	Key string
	Env string
}

// FindSensitiveDefaultsInUse returns every Sensitive=true definition whose
// resolved value in `current` still equals its hardcoded Default. The
// function is pure: it reads no environment variables and touches no
// package state, so callers decide what to do with the result based on
// the runtime environment (see IsDevEnv).
func FindSensitiveDefaultsInUse(
	defs []ConfigDefinition,
	current map[string]interface{},
) []Violation {
	var out []Violation
	for _, d := range defs {
		if !d.Sensitive {
			continue
		}
		if reflect.DeepEqual(current[d.Key], d.Default) {
			out = append(out, Violation{Key: d.Key, Env: d.Env})
		}
	}
	return out
}

// IsDevEnv reports whether goEnv represents a local/development
// environment where hardcoded dev Defaults are tolerated (with a visible
// warning). Recognized values: "" (uninitialized / tests), "dev",
// "development". Any other value — "staging", "qa", "prod", or typos
// like "develpoment" — is treated as non-dev and triggers a fatal abort
// when sensitive defaults are still in use.
func IsDevEnv(goEnv string) bool {
	return goEnv == "" || goEnv == "dev" || goEnv == "development"
}
