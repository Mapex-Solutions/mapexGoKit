package templatereplace

import (
	"fmt"
	"regexp"
	"strings"
)

/*
 * TEMPLATE REPLACE
 * Replaces {{context.field.nested}} patterns in any JSON-like structure.
 *
 * How it works:
 * 1. Finds all {{...}} placeholders in the input
 * 2. Splits the path by "." and navigates the contexts map
 * 3. Replaces the placeholder with the resolved value as string
 *
 * Zero dependencies — pure function, no BSON, no MongoDB.
 */

// regex to find all {{...}} placeholders
var templatePattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Resolve replaces all {{context.field}} templates in any value.
// Walks strings, maps, and slices recursively.
func Resolve(value interface{}, contexts map[string]interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return resolveString(v, contexts)
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for k, inner := range v {
			result[k] = Resolve(inner, contexts)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, inner := range v {
			result[i] = Resolve(inner, contexts)
		}
		return result
	default:
		return value
	}
}

// ResolveString replaces all {{context.field}} in a single string.
func ResolveString(s string, contexts map[string]interface{}) string {
	return resolveString(s, contexts)
}

// resolveString finds all {{...}} in the string and replaces them.
func resolveString(s string, contexts map[string]interface{}) string {
	if !strings.Contains(s, "{{") {
		return s
	}

	return templatePattern.ReplaceAllStringFunc(s, func(match string) string {
		// match = "{{config.chatId}}" → path = "config.chatId"
		path := match[2 : len(match)-2]

		resolved := navigate(path, contexts)
		if resolved == nil {
			return match // unresolved — keep original (e.g., {{before.token}})
		}

		return fmt.Sprintf("%v", resolved)
	})
}

// navigate splits path by "." and walks the contexts map.
// "config.chatId" → contexts["config"]["chatId"]
// "manifest.defaults.baseUrl" → contexts["manifest"]["defaults"]["baseUrl"]
func navigate(path string, contexts map[string]interface{}) interface{} {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}

	current, ok := contexts[parts[0]]
	if !ok {
		return nil
	}

	for _, part := range parts[1:] {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}

	return current
}
