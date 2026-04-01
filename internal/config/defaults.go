package config

// Default configuration values
const (
	DefaultEndpoint        = "http://localhost:8000"
	DefaultRegion          = "us-east-1"
	DefaultAccessKeyID     = "foo"  // LocalStack default
	DefaultSecretAccessKey = "bar"  // LocalStack default
	DefaultOutputFormat    = "json"
)

// GetDefaultTableConfig returns a TableConfig with default values
func GetDefaultTableConfig() TableConfig {
	return TableConfig{
		Endpoint:        DefaultEndpoint,
		Region:          DefaultRegion,
		AccessKeyID:     DefaultAccessKeyID,
		SecretAccessKey: DefaultSecretAccessKey,
	}
}