package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// MigrationSummary contains information about config changes
type MigrationSummary struct {
	FromVersion             int
	ToVersion               int
	HasDeprecatedFields     bool
	DeprecatedFields        []string
	MissingFields           []string
	MissingOptionalSections []string // Top-level optional sections missing (labels, shortcuts, breaks)
	NeedsUpdate             bool
}

// MigrationFunc is a function that migrates config from version N to N+1
type MigrationFunc func(raw map[string]interface{}, summary *MigrationSummary) error

// migrations is the registry of version-specific migration functions
// Each migration bumps the version by 1
var migrations = map[int]MigrationFunc{
	0: migrateV0ToV1, // v0 (no version field) -> v1
}

// MigrateConfig analyzes and migrates a config file to the latest schema
// Returns the migrated content and a summary of changes
func MigrateConfig(data []byte) ([]byte, *MigrationSummary, error) {
	// Parse the YAML into a generic structure
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Detect current version (default to 0 if not present)
	currentVersion := 0
	if v, ok := raw["version"]; ok {
		if vInt, isInt := v.(int); isInt {
			currentVersion = vInt
		}
	}

	// Validate version is not from the future
	if currentVersion > CurrentConfigVersion {
		return nil, nil, fmt.Errorf(
			"config version %d is newer than supported version %d - please upgrade tasklog",
			currentVersion, CurrentConfigVersion,
		)
	}

	// Validate the config matches its declared version (even if no migration needed)
	if err := validateConfigVersion(raw, currentVersion); err != nil {
		return nil, nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Detect missing optional sections
	missingOptionalSections := detectMissingOptionalSections(raw)

	// Check if migration is needed (version upgrade or missing optional sections)
	if currentVersion == CurrentConfigVersion && len(missingOptionalSections) == 0 {
		return data, &MigrationSummary{
			FromVersion: currentVersion,
			ToVersion:   currentVersion,
			NeedsUpdate: false,
		}, nil
	}

	// If only missing optional sections (no version migration needed)
	if currentVersion == CurrentConfigVersion && len(missingOptionalSections) > 0 {
		return data, &MigrationSummary{
			FromVersion:             currentVersion,
			ToVersion:               currentVersion,
			MissingOptionalSections: missingOptionalSections,
			NeedsUpdate:             true,
		}, nil
	}

	summary := &MigrationSummary{
		FromVersion:             currentVersion,
		ToVersion:               CurrentConfigVersion,
		DeprecatedFields:        []string{},
		MissingFields:           []string{},
		MissingOptionalSections: missingOptionalSections,
		NeedsUpdate:             true,
	}

	// Apply migration chain from current version to latest
	for version := currentVersion; version < CurrentConfigVersion; version++ {
		migrationFunc, exists := migrations[version]
		if !exists {
			return nil, nil, fmt.Errorf("no migration function found for version %d to %d", version, version+1)
		}

		if err := migrationFunc(raw, summary); err != nil {
			return nil, nil, fmt.Errorf("failed to migrate from v%d to v%d: %w", version, version+1, err)
		}
	}

	// Set the new version
	raw["version"] = CurrentConfigVersion

	// Marshal back to YAML with added comments for new fields
	updatedYAML, err := marshalWithComments(raw, summary.MissingFields)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal updated config: %w", err)
	}

	return updatedYAML, summary, nil
}

// marshalWithComments marshals the config and adds commented examples for missing fields
func marshalWithComments(raw map[string]interface{}, missingFields []string) ([]byte, error) {
	// First marshal the cleaned config
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, err
	}

	// Parse into yaml.Node to manipulate with comments
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	// Add comments for missing fields
	if err := addMissingFieldComments(&node, missingFields); err != nil {
		return nil, err
	}

	// Marshal with comments preserved
	result, err := yaml.Marshal(&node)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// addMissingFieldComments adds commented examples for missing fields
func addMissingFieldComments(node *yaml.Node, missingFields []string) error {
	if len(missingFields) == 0 {
		return nil
	}

	// Build a set of missing fields for quick lookup
	missingSet := make(map[string]bool)
	for _, field := range missingFields {
		missingSet[field] = true
	}

	// Navigate the YAML tree and add comments
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		rootNode := node.Content[0]
		if rootNode.Kind == yaml.MappingNode {
			addCommentsToMapping(rootNode, missingSet, "")
		}
	}

	return nil
}

// addCommentsToMapping recursively adds comments for missing fields
func addCommentsToMapping(node *yaml.Node, missingFields map[string]bool, prefix string) {
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		key := keyNode.Value
		fullPath := key
		if prefix != "" {
			fullPath = prefix + "." + key
		}

		// Add comments for missing fields in this section
		if key == "jira" && missingFields["jira.task_statuses"] {
			addTaskStatusesComment(valueNode)
		}

		// Recurse into nested mappings
		if valueNode.Kind == yaml.MappingNode {
			addCommentsToMapping(valueNode, missingFields, fullPath)
		}
	}
}

// addTaskStatusesComment adds a comment example for task_statuses
func addTaskStatusesComment(jiraNode *yaml.Node) {
	if jiraNode.Kind != yaml.MappingNode {
		return
	}

	// Check if task_statuses already exists
	for i := 0; i < len(jiraNode.Content); i += 2 {
		if jiraNode.Content[i].Value == "task_statuses" {
			return // Already exists
		}
	}

	// Add commented task_statuses field
	commentNode := &yaml.Node{
		Kind:        yaml.ScalarNode,
		Value:       "task_statuses",
		HeadComment: "Optional: Task statuses to include when fetching tasks (defaults to [\"In Progress\"])\nUncomment and modify as needed:",
	}
	valueNode := &yaml.Node{
		Kind: yaml.SequenceNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "In Progress"},
			{Kind: yaml.ScalarNode, Value: "In Review"},
		},
		LineComment: "Example values - modify as needed",
	}

	jiraNode.Content = append(jiraNode.Content, commentNode, valueNode)
}

// detectMissingOptionalSections compares config against the template structure
// to find missing top-level optional sections automatically.
//
// This function uses the template config (generated from the Config struct) as the
// source of truth. Any new fields added to the Config struct will automatically be
// detected as missing without requiring manual updates to this function.
//
// The approach:
// 1. Generate template config from the Config struct
// 2. Parse both template and user config as maps
// 3. Compare top-level keys, excluding required fields
// 4. Return list of optional keys present in template but missing in user config
//
// Benefits:
// - Automatically detects new optional fields when Config struct is updated
// - No manual maintenance needed when adding new optional sections
// - Single source of truth (the Config struct itself)
func detectMissingOptionalSections(raw map[string]interface{}) []string {
	var missing []string

	// Generate template to get the complete structure
	templateData, err := GenerateExampleConfig()
	if err != nil {
		// If we can't generate template, return empty (fail-safe)
		return missing
	}

	var templateRaw map[string]interface{}
	if err := yaml.Unmarshal(templateData, &templateRaw); err != nil {
		return missing
	}

	// Define required fields that must not be reported as missing
	// These are validated separately by Config.Validate()
	// Note: Only truly required fields are listed here.
	// slack and database are optional - they should be detected as missing if absent.
	requiredFields := map[string]bool{
		"version": true,
		"jira":    true,
		"tempo":   true,
	}

	// Compare top-level keys between template and user config
	for key := range templateRaw {
		// Skip required fields
		if requiredFields[key] {
			continue
		}

		// If user config is missing this optional key, add to missing list
		if _, exists := raw[key]; !exists {
			missing = append(missing, key)
		}
	}

	return missing
}

// ApplyOptionalSections adds missing optional sections from the template to user's config
func ApplyOptionalSections(userConfig []byte, missingSections []string) ([]byte, error) {
	if len(missingSections) == 0 {
		return userConfig, nil
	}

	// Generate template config to get example values
	templateData, err := GenerateExampleConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to generate template: %w", err)
	}

	// Parse both configs
	var userRaw map[string]interface{}
	if err := yaml.Unmarshal(userConfig, &userRaw); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	var templateRaw map[string]interface{}
	if err := yaml.Unmarshal(templateData, &templateRaw); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Copy missing sections from template to user config
	for _, section := range missingSections {
		if value, exists := templateRaw[section]; exists {
			userRaw[section] = value
		}
	}

	// Marshal back to YAML preserving structure
	result, err := yaml.Marshal(userRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	return result, nil
}

// VersionValidator validates that a config matches its declared version
type VersionValidator func(raw map[string]interface{}) error

// versionValidators maps version numbers to their validation functions
var versionValidators = map[int]VersionValidator{
	0: validateV0Config,
	1: validateV1Config,
}

// validateConfigVersion validates that the config structure matches its declared version
func validateConfigVersion(raw map[string]interface{}, version int) error {
	validator, exists := versionValidators[version]
	if !exists {
		// No validator means we accept it (for forward compatibility within same major version)
		return nil
	}
	return validator(raw)
}

// validateV0Config validates v0 (legacy) config structure
// V0 configs may have user_token and may be missing task_statuses
func validateV0Config(raw map[string]interface{}) error {
	// V0 is lenient - just check basic structure exists
	if _, hasJira := raw["jira"]; !hasJira {
		return fmt.Errorf("v0 config must have 'jira' section")
	}

	return nil
}

// validateV1Config validates v1 config structure
func validateV1Config(raw map[string]interface{}) error {
	// Basic structure check - jira section must exist
	if _, hasJira := raw["jira"]; !hasJira {
		return fmt.Errorf("v1 config must have 'jira' section")
	}

	// Don't validate all required fields here - that's done by Config.Validate() after loading
	// This validation is just to catch version mismatches
	return nil
}

// migrateV0ToV1 migrates config from v0 (no version) to v1
// Changes:
// - Adds jira.task_statuses (as comment if missing)
// - No changes to Slack fields (user_token is still valid)
//
// Note on nested optional fields:
// Nested optional fields within required sections (like jira.task_statuses) are
// handled here in version-specific migration functions. This is intentional because:
// 1. These fields may have version-specific behavior or defaults
// 2. Migration functions can add them with appropriate comments/examples
// 3. They're tied to specific version changes in the schema
//
// Top-level optional sections (labels, shortcuts, breaks) are detected automatically
// by detectMissingOptionalSections() without manual updates.
func migrateV0ToV1(raw map[string]interface{}, summary *MigrationSummary) error {
	// Check for missing jira.task_statuses
	if jiraSection, ok := raw["jira"].(map[string]interface{}); ok {
		if _, hasTaskStatuses := jiraSection["task_statuses"]; !hasTaskStatuses {
			summary.MissingFields = append(summary.MissingFields, "jira.task_statuses")
		}
	}

	return nil
}
