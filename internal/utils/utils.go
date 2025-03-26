package utils

import (
	"strings"
	"unicode"
)

// TruncateAndDashCase converts a string to dash-case and truncates it to the specified length
func TruncateAndDashCase(s string, maxLen int) string {
	// Convert to dash case
	var result strings.Builder
	var lastWasDash bool

	for i, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			// Replace non-alphanumeric with dash
			if !lastWasDash && (i > 0 || unicode.IsLetter(r) || unicode.IsDigit(r)) {
				result.WriteRune('-')
				lastWasDash = true
			}
		} else if i > 0 && unicode.IsUpper(r) && !lastWasDash {
			// Add dash before uppercase letters (camelCase to dash-case)
			result.WriteRune('-')
			result.WriteRune(unicode.ToLower(r))
			lastWasDash = false
		} else {
			// Add lowercase version of the character
			result.WriteRune(unicode.ToLower(r))
			lastWasDash = false
		}
	}

	// Truncate if needed
	resultStr := result.String()
	// Remove trailing dash if present
	if len(resultStr) > 0 && resultStr[len(resultStr)-1] == '-' {
		resultStr = resultStr[:len(resultStr)-1]
	}

	if len(resultStr) > maxLen {
		return resultStr[:maxLen]
	}

	return resultStr
}

// SplitOnCommaAndWhitespace splits a string on commas and trims whitespace
func SplitOnCommaAndWhitespace(s string) []string {
	parts := strings.Split(s, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}
