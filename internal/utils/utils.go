package utils

import (
	"fmt"
	"strings"
	"time"
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

// GenerateMilestone creates a milestone string in the format YYYY.QN based on the provided time
// For example: 2025.Q1 for January-March, 2025.Q2 for April-June, etc.
func GenerateMilestone(t time.Time) string {
	year := t.Year()
	month := t.Month()

	var quarter int
	switch {
	case month >= time.January && month <= time.March:
		quarter = 1
	case month >= time.April && month <= time.June:
		quarter = 2
	case month >= time.July && month <= time.September:
		quarter = 3
	case month >= time.October && month <= time.December:
		quarter = 4
	}

	return fmt.Sprintf("%d.Q%d", year, quarter)
}
