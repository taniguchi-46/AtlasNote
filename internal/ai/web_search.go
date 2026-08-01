package ai

import (
	"net"
	"net/url"
	"strings"
)

const (
	aiMaxWebCitations     = 5
	aiMaxCitationURLBytes = 4 * 1024
	aiMaxCitationTitle    = 200
)

func normalizeWebCitations(values []WebCitation) []WebCitation {
	result := make([]WebCitation, 0, minInt(len(values), aiMaxWebCitations))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		rawURL := strings.TrimSpace(value.URL)
		if rawURL == "" || len([]byte(rawURL)) > aiMaxCitationURLBytes {
			continue
		}
		parsed, err := url.ParseRequestURI(rawURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			continue
		}
		host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
		if isUnsafeCitationHost(host) {
			continue
		}
		normalizedURL := parsed.String()
		if _, exists := seen[normalizedURL]; exists {
			continue
		}
		seen[normalizedURL] = struct{}{}
		result = append(result, WebCitation{
			URL:   normalizedURL,
			Title: limitUTF8Bytes(strings.TrimSpace(value.Title), aiMaxCitationTitle),
		})
		if len(result) >= aiMaxWebCitations {
			break
		}
	}
	return result
}

func isUnsafeCitationHost(host string) bool {
	if host == "" ||
		!strings.Contains(host, ".") ||
		host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".lan") ||
		strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".home") ||
		strings.HasSuffix(host, ".home.arpa") {
		return true
	}
	// Search citations should resolve to ordinary public hostnames. Reject all
	// IP literals and numeric/hex IPv4 spellings that browsers normalize only
	// after Go's URL validation has completed.
	return net.ParseIP(host) != nil || isEncodedIPAddressHost(host)
}

func isEncodedIPAddressHost(host string) bool {
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" {
			return false
		}
		value := strings.ToLower(label)
		if strings.HasPrefix(value, "0x") {
			if len(value) == 2 || !allASCIIHex(value[2:]) {
				return false
			}
			continue
		}
		if !allASCIIDigits(value) {
			return false
		}
	}
	return true
}

func allASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func allASCIIHex(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
