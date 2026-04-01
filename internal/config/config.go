package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	
	"querydb/internal/errors"
)

// Config represents the main configuration structure
type Config struct {
	Tables map[string]TableConfig `yaml:"tables"`
}

// TableConfig represents configuration for a specific DynamoDB table
type TableConfig struct {
	TableName       string `yaml:"table_name"`
	Endpoint        string `yaml:"endpoint"`
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		Tables: make(map[string]TableConfig),
	}
}

// LoadConfig loads configuration from the specified file path
// If the file doesn't exist, it creates the directory structure and a sample config
func LoadConfig(configPath string) (*Config, error) {
	// Expand home directory if needed
	expandedPath, err := homedir.Expand(configPath)
	if err != nil {
		return nil, errors.NewConfigurationError(
			"Failed to expand config path",
			err,
			"Check if the config path is valid",
			"Ensure the home directory is accessible",
		)
	}

	// Check if config file exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		// Create directory structure and sample config
		if err := createSampleConfig(expandedPath); err != nil {
			return nil, errors.NewConfigurationError(
				"Failed to create sample config file",
				err,
				"Check if you have write permissions to the config directory",
				"Ensure the parent directory exists and is writable",
				"Try creating the directory manually: mkdir -p ~/.config/querydb",
			)
		}
	}

	// Load the configuration file
	config := DefaultConfig()
	
	// Read the file
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, errors.NewConfigurationError(
			"Failed to read config file",
			err,
			"Check if the config file exists and is readable",
			"Verify file permissions",
			fmt.Sprintf("Config file path: %s", expandedPath),
		)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, errors.NewConfigurationError(
			"Failed to parse config file - invalid YAML syntax",
			err,
			"Check the YAML syntax in your config file",
			"Ensure proper indentation and structure",
			"Use a YAML validator to check for syntax errors",
			fmt.Sprintf("Config file path: %s", expandedPath),
		)
	}

	return config, nil
}

// createSampleConfig creates the directory structure and a sample configuration file
func createSampleConfig(configPath string) error {
	// Create directory structure
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create comprehensive sample configuration with detailed comments
	sampleConfigContent := `# QueryDB Configuration File
# =========================
# This file contains table configurations for the QueryDB CLI tool.
# Each table configuration specifies connection parameters and credentials
# for DynamoDB tables, allowing you to query them by name without
# repeatedly specifying connection details.
#
# USAGE:
#   querydb <table-config-name>           # Use configuration from this file
#   querydb --table <name> --endpoint ... # Use command-line flags
#   querydb <config-name> --endpoint ...  # Mix config and flags (flags override)
#
# CONFIGURATION STRUCTURE:
#   tables:
#     <config-name>:                      # Friendly name for your table config
#       table_name: <actual-table-name>   # Real DynamoDB table name
#       endpoint: <dynamodb-endpoint>     # DynamoDB endpoint URL
#       region: <aws-region>              # AWS region
#       access_key_id: <access-key>       # AWS access key (optional for LocalStack)
#       secret_access_key: <secret-key>   # AWS secret key (optional for LocalStack)

tables:
  # Example configuration for a products table
  products-table:
    table_name: "Products"
    endpoint: "http://localhost:8000"     # LocalStack DynamoDB endpoint
    region: "us-east-1"                   # Default AWS region
    access_key_id: "foo"                  # LocalStack default credentials
    secret_access_key: "bar"              # LocalStack default credentials

  # Example configuration for a users table with different endpoint
  users-table:
    table_name: "users"
    endpoint: "http://localhost:4566"     # Alternative LocalStack port
    region: "us-west-2"                   # Different region example
    # access_key_id and secret_access_key are optional
    # If not specified, defaults to LocalStack credentials (foo/bar)

  # Example configuration for production AWS DynamoDB
  prod-users:
    table_name: "production-users-table"
    endpoint: "https://dynamodb.us-east-1.amazonaws.com"  # Real AWS endpoint
    region: "us-east-1"
    # For production, use AWS credentials from environment variables,
    # AWS credentials file, or IAM roles instead of hardcoding here
    # access_key_id: "your-aws-access-key"
    # secret_access_key: "your-aws-secret-key"

# CONFIGURATION OPTIONS EXPLAINED:
#
# table_name (required):
#   The actual name of the DynamoDB table as it exists in AWS/LocalStack
#
# endpoint (required):
#   - For LocalStack: http://localhost:8000 (default) or http://localhost:4566
#   - For AWS: https://dynamodb.<region>.amazonaws.com
#   - Default if not specified: http://localhost:8000
#
# region (required):
#   AWS region where the table is located
#   Default if not specified: us-east-1
#
# access_key_id (optional):
#   AWS access key ID for authentication
#   Default if not specified: "foo" (LocalStack default)
#
# secret_access_key (optional):
#   AWS secret access key for authentication
#   Default if not specified: "bar" (LocalStack default)
#
# SECURITY NOTE:
#   Avoid storing production AWS credentials in this file.
#   Use environment variables, AWS credentials file, or IAM roles instead.
#
# EXAMPLES OF USAGE:
#   querydb products-table                    # Query using products-table config
#   querydb users-table                         # Query using users-table config
#   querydb prod-users                          # Query using prod-users config
#   querydb products-table --region us-west-1 # Override region from config
`

	// Write to file
	if err := os.WriteFile(configPath, []byte(sampleConfigContent), 0644); err != nil {
		return fmt.Errorf("failed to write sample config file: %w", err)
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "querydb", "config.yaml"), nil
}

// GetTableConfig retrieves a table configuration by name
func (c *Config) GetTableConfig(name string) (TableConfig, bool) {
	config, exists := c.Tables[name]
	return config, exists
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// Validate validates the configuration and returns any validation errors
func (c *Config) Validate() []ValidationError {
	var errors []ValidationError

	for tableName, tableConfig := range c.Tables {
		// Validate required fields
		if tableConfig.TableName == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("tables.%s.table_name", tableName),
				Message: "table_name is required",
			})
		}

		if tableConfig.Endpoint == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("tables.%s.endpoint", tableName),
				Message: "endpoint is required",
			})
		}

		if tableConfig.Region == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("tables.%s.region", tableName),
				Message: "region is required",
			})
		}
	}

	return errors
}

// ApplyDefaults applies default values to table configurations
func (tc *TableConfig) ApplyDefaults() {
	if tc.Endpoint == "" {
		tc.Endpoint = DefaultEndpoint
	}
	if tc.Region == "" {
		tc.Region = DefaultRegion
	}
	// Set default LocalStack credentials if not provided
	if tc.AccessKeyID == "" {
		tc.AccessKeyID = DefaultAccessKeyID
	}
	if tc.SecretAccessKey == "" {
		tc.SecretAccessKey = DefaultSecretAccessKey
	}
}

// MergedTableConfig represents a table configuration merged with CLI flags
type MergedTableConfig struct {
	TableName       string
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

// MergeWithFlags merges table configuration with CLI flags (flags take precedence)
func (tc *TableConfig) MergeWithFlags(flags map[string]interface{}) *MergedTableConfig {
	merged := &MergedTableConfig{
		TableName:       tc.TableName,
		Endpoint:        tc.Endpoint,
		Region:          tc.Region,
		AccessKeyID:     tc.AccessKeyID,
		SecretAccessKey: tc.SecretAccessKey,
	}

	// Apply CLI flags if provided (they override config values)
	if table, ok := flags["table"].(string); ok && table != "" {
		merged.TableName = table
	}
	if endpoint, ok := flags["endpoint"].(string); ok && endpoint != "" {
		merged.Endpoint = endpoint
	}
	if region, ok := flags["region"].(string); ok && region != "" {
		merged.Region = region
	}
	if accessKey, ok := flags["access_key"].(string); ok && accessKey != "" {
		merged.AccessKeyID = accessKey
	}
	if secretKey, ok := flags["secret_key"].(string); ok && secretKey != "" {
		merged.SecretAccessKey = secretKey
	}

	return merged
}

// CreateFallbackConfig creates a configuration from CLI flags when no config file entry exists
func CreateFallbackConfig(flags map[string]interface{}) *MergedTableConfig {
	config := &MergedTableConfig{
		Endpoint:        DefaultEndpoint,        // default
		Region:          DefaultRegion,          // default
		AccessKeyID:     DefaultAccessKeyID,     // default LocalStack
		SecretAccessKey: DefaultSecretAccessKey, // default LocalStack
	}

	// Apply CLI flags
	if table, ok := flags["table"].(string); ok {
		config.TableName = table
	}
	if endpoint, ok := flags["endpoint"].(string); ok && endpoint != "" {
		config.Endpoint = endpoint
	}
	if region, ok := flags["region"].(string); ok && region != "" {
		config.Region = region
	}
	if accessKey, ok := flags["access_key"].(string); ok && accessKey != "" {
		config.AccessKeyID = accessKey
	}
	if secretKey, ok := flags["secret_key"].(string); ok && secretKey != "" {
		config.SecretAccessKey = secretKey
	}

	return config
}

// ValidateMergedConfig validates a merged configuration
func (mc *MergedTableConfig) Validate() error {
	if mc.TableName == "" {
		return fmt.Errorf("table name is required (use --table flag or provide table config name)")
	}
	if mc.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if mc.Region == "" {
		return fmt.Errorf("region is required")
	}
	return nil
}

// FlagChangedFunc is a function that reports whether a flag was explicitly set on the CLI.
// It is set by the cmd package at init time so the config package can distinguish
// user-supplied flags from viper defaults.
var FlagChangedFunc func(name string) bool

// GetViperFlags extracts relevant flags from Viper into a map.
// Only flags explicitly set by the user are included; viper defaults are omitted
// so they don't override values from the config file.
func GetViperFlags() map[string]interface{} {
	flags := make(map[string]interface{})

	changed := FlagChangedFunc
	if changed == nil {
		// No flag-changed checker registered; treat nothing as explicitly set
		return flags
	}

	if changed("table") {
		flags["table"] = viper.GetString("table")
	}
	if changed("endpoint") {
		flags["endpoint"] = viper.GetString("endpoint")
	}
	if changed("region") {
		flags["region"] = viper.GetString("region")
	}
	if changed("access-key") {
		flags["access_key"] = viper.GetString("access_key")
	}
	if changed("secret-key") {
		flags["secret_key"] = viper.GetString("secret_key")
	}

	return flags
}

// ResolveTableConfig resolves table configuration by name or falls back to CLI flags
func (c *Config) ResolveTableConfig(tableConfigName string) (*MergedTableConfig, error) {
	flags := GetViperFlags()

	if tableConfigName != "" {
		// Try to find table configuration by name
		if tableConfig, exists := c.GetTableConfig(tableConfigName); exists {
			// Apply defaults to the table config
			tableConfig.ApplyDefaults()
			
			// Validate the table config
			if validationErrors := c.validateSingleTableConfig(tableConfigName, tableConfig); len(validationErrors) > 0 {
				var errorMessages []string
				for _, ve := range validationErrors {
					errorMessages = append(errorMessages, ve.Error())
				}
				return nil, errors.NewValidationError(
					"Table configuration validation failed",
					fmt.Errorf("%s", strings.Join(errorMessages, "; ")),
					"Check your config file for missing or invalid fields",
					"Ensure all required fields (table_name, endpoint, region) are present",
					fmt.Sprintf("Table config name: %s", tableConfigName),
				)
			}
			
			// Merge with CLI flags (flags override config)
			return tableConfig.MergeWithFlags(flags), nil
		} else {
			// Get available table names for suggestions
			var availableNames []string
			for name := range c.Tables {
				availableNames = append(availableNames, name)
			}
			
			suggestions := []string{
				"Check the spelling of the table configuration name",
				"Use --table flag to specify table name directly",
			}
			if len(availableNames) > 0 {
				suggestions = append(suggestions, fmt.Sprintf("Available configurations: %s", strings.Join(availableNames, ", ")))
			}
			
			return nil, errors.NewConfigurationError(
				fmt.Sprintf("Table configuration '%s' not found in config file", tableConfigName),
				nil,
				suggestions...,
			)
		}
	}

	// No table config name provided, use CLI flags only
	config := CreateFallbackConfig(flags)
	
	// Validate the merged config
	if err := config.Validate(); err != nil {
		return nil, errors.NewValidationError(
			"Configuration validation failed",
			err,
			"Provide table name using --table flag",
			"Ensure endpoint and region are specified",
			"Use --help to see all available options",
		)
	}

	return config, nil
}

// validateSingleTableConfig validates a single table configuration
func (c *Config) validateSingleTableConfig(tableName string, tableConfig TableConfig) []ValidationError {
	var errors []ValidationError

	if tableConfig.TableName == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("tables.%s.table_name", tableName),
			Message: "table_name is required",
		})
	}

	if tableConfig.Endpoint == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("tables.%s.endpoint", tableName),
			Message: "endpoint is required",
		})
	}

	if tableConfig.Region == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("tables.%s.region", tableName),
			Message: "region is required",
		})
	}

	return errors
}