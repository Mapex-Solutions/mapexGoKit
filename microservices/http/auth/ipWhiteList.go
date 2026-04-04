package auth

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ValidateIPWhitelist checks whether the client's IP address is included in the list of allowed IPs.
//
// Parameters:
//   - c: the Fiber context, used to retrieve the client's IP address.
//   - allowedIPs: a slice of allowed IP addresses as strings.
//
// Returns:
//   - bool: true if the client's IP matches any of the allowed IPs; false otherwise.
//
// The function compares the client's IP address obtained from the Fiber context against
// the provided whitelist. Whitespace around each allowed IP is trimmed before comparison.
// ValidateIPWhitelist checks if the request's client IP is allowed.
// Supports: exact IPs (v4/v6), CIDR ranges, "localhost"/"loopback", and hostnames.
func ValidateIPWhitelist(c *fiber.Ctx, allowedIPs []string) bool {
	clientIP := net.ParseIP(c.IP())
	if clientIP == nil {
		return false
	}

	// Normalize IPv4-mapped IPv6 (e.g., ::ffff:127.0.0.1) to pure IPv4 for comparisons.
	if v4 := clientIP.To4(); v4 != nil {
		clientIP = v4
	}

	for _, raw := range allowedIPs {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		// Special cases: allow any loopback (127.0.0.0/8 and ::1)
		if entry == "localhost" || entry == "loopback" {
			if clientIP.IsLoopback() {
				return true
			}
		}

		// CIDR range (e.g., "192.168.0.0/24", "127.0.0.0/8", "::1/128")
		if _, ipNet, err := net.ParseCIDR(entry); err == nil {
			if ipNet.Contains(clientIP) {
				return true
			}
			continue
		}

		// Exact IP (v4/v6)
		if parsed := net.ParseIP(entry); parsed != nil {
			// Normalize mapped v4 for fair equality
			if v4 := parsed.To4(); v4 != nil {
				parsed = v4
			}
			if parsed.Equal(clientIP) {
				return true
			}
			continue
		}

		// Hostname -> resolve and compare
		if hasLetters(entry) {
			if ips, _ := net.LookupIP(entry); len(ips) > 0 {
				for _, ip := range ips {
					if v4 := ip.To4(); v4 != nil {
						ip = v4
					}
					if ip.Equal(clientIP) {
						return true
					}
				}
			}
		}
	}

	return false
}

// hasLetters is a tiny helper to detect hostnames (very permissive).
func hasLetters(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
			return true
		}
	}
	return false
}
