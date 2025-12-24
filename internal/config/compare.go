package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComparisonResult contains the differences between two configs
type ComparisonResult struct {
	MissingKeys []string // Keys in example but not in user config
	ExtraKeys   []string // Keys in user config but not in example
	IsUpToDate  bool     // True if no missing keys
}

// CompareWithExample compares user's config with the example config
func CompareWithExample(userConfigData []byte) (*ComparisonResult, error) {
	// Generate example config
	exampleData, err := GenerateExampleConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to generate example config: %w", err)
	}

	// Parse both configs into generic maps
	var userConfig map[string]interface{}
	if err := yaml.Unmarshal(userConfigData, &userConfig); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	var exampleConfig map[string]interface{}
	if err := yaml.Unmarshal(exampleData, &exampleConfig); err != nil {
		return nil, fmt.Errorf("failed to parse example config: %w", err)
	}

	result := &ComparisonResult{
		MissingKeys: []string{},
		ExtraKeys:   []string{},
	}

	// Find keys in example but not in user config
	findMissingKeys(exampleConfig, userConfig, "", &result.MissingKeys)

	// Find keys in user config but not in example (optional - shows deprecated/custom fields)
	findExtraKeys(userConfig, exampleConfig, "", &result.ExtraKeys)

	result.IsUpToDate = len(result.MissingKeys) == 0

	return result, nil
}

// findMissingKeys recursively finds keys that exist in example but not in user config
func findMissingKeys(example, user map[string]interface{}, prefix string, missing *[]string) {
	for key, exampleValue := range example {
		currentPath := key
		if prefix != "" {
			currentPath = prefix + "." + key
		}

		userValue, exists := user[key]

		// If key doesn't exist in user config, it's missing
		if !exists {
			*missing = append(*missing, currentPath)
			continue
		}

		// If both are maps, recurse
		exampleMap, exampleIsMap := exampleValue.(map[string]interface{})
		userMap, userIsMap := userValue.(map[string]interface{})

		if exampleIsMap && userIsMap {
			findMissingKeys(exampleMap, userMap, currentPath, missing)
		}
	}
}

// findExtraKeys recursively finds keys that exist in user config but not in example
func findExtraKeys(user, example map[string]interface{}, prefix string, extra *[]string) {
	for key, userValue := range user {
		currentPath := key
		if prefix != "" {
			currentPath = prefix + "." + key
		}

		exampleValue, exists := example[key]

		// If key doesn't exist in example, it's extra (custom or deprecated)
		if !exists {
			*extra = append(*extra, currentPath)
			continue
		}

		// If both are maps, recurse
		userMap, userIsMap := userValue.(map[string]interface{})
		exampleMap, exampleIsMap := exampleValue.(map[string]interface{})

		if userIsMap && exampleIsMap {
			findExtraKeys(userMap, exampleMap, currentPath, extra)
		}
	}
}

// FormatComparisonResult returns a formatted string of the comparison results
func FormatComparisonResult(result *ComparisonResult) string {
	var output strings.Builder

	if result.IsUpToDate && len(result.ExtraKeys) == 0 {
		output.WriteString("✓ Your configuration is up to date!\n\n")
		output.WriteString("All fields from the example config are present.\n")
		return output.String()
	}

	if len(result.MissingKeys) > 0 {
		output.WriteString("📋 Missing fields (available in example config):\n\n")
		for _, key := range result.MissingKeys {
			output.WriteString(fmt.Sprintf("   • %s\n", key))
		}
		output.WriteString("\n💡 Run 'tasklog config example' to see what these fields do.\n")
	}

	if len(result.ExtraKeys) > 0 {
		if len(result.MissingKeys) > 0 {
			output.WriteString("\n")
		}
		output.WriteString("⚠️  Extra fields (not in example config):\n\n")
		for _, key := range result.ExtraKeys {
			output.WriteString(fmt.Sprintf("   • %s\n", key))
		}
		output.WriteString("\n   These might be custom fields or deprecated.\n")
	}

	return output.String()
}
