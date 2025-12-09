package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// TruncateAndDashCase converts a string to dash-case and truncates it to the specified length
// Keeps consecutive uppercase letters together (e.g., "IRSA" stays as "irsa", not "i-r-s-a")
func TruncateAndDashCase(s string, maxLen int) string {
	// Convert to dash case
	var result strings.Builder
	var lastWasDash bool
	runes := []rune(s)

	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			// Replace non-alphanumeric with dash
			if !lastWasDash && i > 0 {
				result.WriteRune('-')
				lastWasDash = true
			}
		} else if unicode.IsUpper(r) {
			// Determine if we need a dash before this uppercase letter
			needsDash := false

			if i > 0 && !lastWasDash {
				prevChar := runes[i-1]
				prevIsLetter := unicode.IsLetter(prevChar)

				if prevIsLetter {
					// Check if next character exists and is a letter
					hasNextLetter := i < len(runes)-1 && unicode.IsLetter(runes[i+1])
					nextIsUpper := hasNextLetter && unicode.IsUpper(runes[i+1])
					nextIsLower := hasNextLetter && unicode.IsLower(runes[i+1])
					prevIsUpper := unicode.IsUpper(prevChar)

					if prevIsUpper && nextIsUpper {
						// Middle of acronym: ABC -> abc (no dash)
						needsDash = false
					} else if prevIsUpper && nextIsLower {
						// End of acronym before lowercase: ABCdef -> abc-def
						needsDash = true
					} else if !prevIsUpper {
						// Transition from lowercase to uppercase: abcDef -> abc-def
						needsDash = true
					}
					// If prevIsUpper and no next letter (end of acronym), no dash
				}
			}

			if needsDash {
				result.WriteRune('-')
			}
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

// ExtractIssueInfo extracts the project name (optional) and issue number from a branch name.
// Branch names can be in the format:
//   - 123-branch-name (same repo)
//   - project-123-branch-name (cross-repo)
//
// Returns: (projectName, issueNumber, error)
// If projectName is empty, the issue is in the same repo as the branch.
func ExtractIssueInfo(branchName string) (string, int, error) {
	// Split the branch name by dash
	parts := strings.Split(branchName, "-")
	if len(parts) < 2 {
		return "", 0, fmt.Errorf("invalid branch name format: %s", branchName)
	}

	// Check if first part is numeric (same-repo format: 123-foo)
	if issueNumber, err := strconv.Atoi(parts[0]); err == nil {
		return "", issueNumber, nil
	}

	// Check if second part is numeric (cross-repo format: project-123-foo)
	if len(parts) < 3 {
		return "", 0, fmt.Errorf("invalid branch name format: %s", branchName)
	}

	issueNumber, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid branch name format: %s", branchName)
	}

	return parts[0], issueNumber, nil
}

// ExtractIssueNumber extracts the issue number from a branch name.
// Branch names are typically in the format 123-branch-name or project-123-branch-name.
// This function is kept for backward compatibility and uses ExtractIssueInfo internally.
func ExtractIssueNumber(branchName string) (int, error) {
	_, issueNumber, err := ExtractIssueInfo(branchName)
	return issueNumber, err
}
