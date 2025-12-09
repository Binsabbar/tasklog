package prerelease

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigIssue represents a configuration issue found during validation
type ConfigIssue struct {
	Field       string // Path to the field (e.g., "update.check_for_updates")
	Issue       string // Description of the issue
	Suggestion  string // How to fix it
	Severity    string // "error" or "warning"
	ReleaseNote string // Which pre-release introduced this change
}

type KnownIssue struct {
	Field      string
	OldName    string
	NewName    string
	Severity   string
	Release    string // Which release introduced the change
	LogicFlip  bool   // Whether the boolean logic was inverted
	Suggestion string
}

// KnownIssues tracks breaking changes in pre-release versions
// This is a living document that accumulates changes across alpha/beta/rc releases
var KnownIssues = []KnownIssue{
	{
		Field:      "update.check_for_updates",
		OldName:    "check_for_updates",
		NewName:    "disabled",
		Severity:   "high",
		Release:    "v1.0.0-alpha.5",
		LogicFlip:  true,
		Suggestion: "Replace 'check_for_updates: true' with 'disabled: false' (logic is inverted)",
	},
	{
		Field:      "shortcuts",
		OldName:    "shortcuts",
		NewName:    "jira.shortcuts",
		Severity:   "high",
		Release:    "v1.0.0-alpha.6",
		LogicFlip:  false,
		Suggestion: "Move 'shortcuts' array from root level to under 'jira:' section",
	},
	{
		Field:      "breaks",
		OldName:    "breaks",
		NewName:    "slack.breaks",
		Severity:   "high",
		Release:    "v1.0.0-alpha.6",
		LogicFlip:  false,
		Suggestion: "Move 'breaks' array from root level to under 'slack:' section",
	},
}

// ValidateConfig checks a config file for known pre-release breaking changes
// Returns a list of issues found
func ValidateConfig(configData []byte) ([]ConfigIssue, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(configData, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	var issues []ConfigIssue

	// Check each known issue
	for _, known := range KnownIssues {
		if issue := checkField(raw, known); issue != nil {
			issues = append(issues, *issue)
		}
	}

	return issues, nil
}

// checkField checks if a specific deprecated field exists in the config
func checkField(raw map[string]interface{}, known KnownIssue) *ConfigIssue {
	// Parse field path (e.g., "update.check_for_updates" -> ["update", "check_for_updates"])
	parts := strings.Split(known.Field, ".")

	// For root-level fields (like "shortcuts" or "breaks")
	if len(parts) == 1 {
		if _, exists := raw[known.OldName]; exists {
			return &ConfigIssue{
				Field:       known.Field,
				Issue:       fmt.Sprintf("Deprecated field '%s' found at root level (moved in %s)", known.OldName, known.Release),
				Suggestion:  known.Suggestion,
				Severity:    known.Severity,
				ReleaseNote: known.Release,
			}
		}
		return nil
	}

	// Navigate to the parent section for nested fields
	current := raw
	for i := range len(parts) - 1 { //nolint:intrange // using traditional loop for clarity
		next, ok := current[parts[i]].(map[string]interface{})
		if !ok {
			return nil // Section doesn't exist
		}
		current = next
	}

	// Check if the old field exists
	fieldName := parts[len(parts)-1]
	if _, exists := current[fieldName]; exists {
		return &ConfigIssue{
			Field:       known.Field,
			Issue:       fmt.Sprintf("Deprecated field '%s' found (renamed in %s)", known.OldName, known.Release),
			Suggestion:  known.Suggestion,
			Severity:    known.Severity,
			ReleaseNote: known.Release,
		}
	}

	return nil
}

// FormatIssues formats issues for display to the user
func FormatIssues(issues []ConfigIssue) string {
	if len(issues) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n⚠️  Configuration issues detected in your config file:\n\n")

	for i, issue := range issues {
		sb.WriteString(fmt.Sprintf("%d. Field: %s\n", i+1, issue.Field))
		sb.WriteString(fmt.Sprintf("   Issue: %s\n", issue.Issue))
		sb.WriteString(fmt.Sprintf("   Fix: %s\n", issue.Suggestion))
		if i < len(issues)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
