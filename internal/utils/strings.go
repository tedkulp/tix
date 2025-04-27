package utils

import "strings"

// Contains checks if a string contains another string
func Contains(s, substring string) bool {
	return strings.Contains(s, substring)
}
