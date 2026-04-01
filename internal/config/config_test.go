package config

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {

	Describe("DefaultConfig", func() {
		It("should return a non-nil config with empty tables", func() {
			config := DefaultConfig()
			Expect(config).NotTo(BeNil())
			Expect(config.Tables).NotTo(BeNil())
			Expect(config.Tables).To(BeEmpty())
		})
	})

	Describe("LoadConfig", func() {
		Context("when the config file exists", func() {
			It("should load and parse the config correctly", func() {
				tempDir := GinkgoT().TempDir()
				configPath := filepath.Join(tempDir, "config.yaml")

				configContent := `tables:
  test-table:
    table_name: "test-dynamodb-table"
    endpoint: "http://localhost:8000"
    region: "us-east-1"
    access_key_id: "test-key"
    secret_access_key: "test-secret"
`

				err := os.WriteFile(configPath, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				config, err := LoadConfig(configPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).NotTo(BeNil())

				Expect(config.Tables).To(HaveLen(1))
				tableConfig, exists := config.Tables["test-table"]
				Expect(exists).To(BeTrue())
				Expect(tableConfig.TableName).To(Equal("test-dynamodb-table"))
				Expect(tableConfig.Endpoint).To(Equal("http://localhost:8000"))
				Expect(tableConfig.Region).To(Equal("us-east-1"))
				Expect(tableConfig.AccessKeyID).To(Equal("test-key"))
				Expect(tableConfig.SecretAccessKey).To(Equal("test-secret"))
			})
		})

		Context("when the config file does not exist", func() {
			It("should create a sample config and return it", func() {
				tempDir := GinkgoT().TempDir()
				configPath := filepath.Join(tempDir, "nonexistent", "config.yaml")

				config, err := LoadConfig(configPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).NotTo(BeNil())

				_, statErr := os.Stat(configPath)
				Expect(statErr).NotTo(HaveOccurred())

				Expect(config.Tables).To(HaveLen(3))

				productsTable, exists := config.Tables["products-table"]
				Expect(exists).To(BeTrue())
				Expect(productsTable.TableName).To(Equal("Products"))
				Expect(productsTable.Endpoint).To(Equal("http://localhost:8000"))
				Expect(productsTable.Region).To(Equal("us-east-1"))
				Expect(productsTable.AccessKeyID).To(Equal("foo"))
				Expect(productsTable.SecretAccessKey).To(Equal("bar"))

				usersTable, exists := config.Tables["users-table"]
				Expect(exists).To(BeTrue())
				Expect(usersTable.TableName).To(Equal("users"))
				Expect(usersTable.Endpoint).To(Equal("http://localhost:4566"))
				Expect(usersTable.Region).To(Equal("us-west-2"))
			})
		})

		Context("when the config file has invalid YAML", func() {
			It("should return an error", func() {
				tempDir := GinkgoT().TempDir()
				configPath := filepath.Join(tempDir, "config.yaml")

				invalidYAML := `tables:
  test-table:
    table_name: "test-table"
    endpoint: http://localhost:8000
    region: us-east-1
    invalid_yaml: [unclosed bracket
`
				err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
				Expect(err).NotTo(HaveOccurred())

				config, err := LoadConfig(configPath)
				Expect(err).To(HaveOccurred())
				Expect(config).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("Failed to parse config file"))
			})
		})
	})

	Describe("GetTableConfig", func() {
		It("should return the table config when it exists", func() {
			config := &Config{
				Tables: map[string]TableConfig{
					"test-table": {
						TableName: "test-dynamodb-table",
						Endpoint:  "http://localhost:8000",
						Region:    "us-east-1",
					},
				},
			}
			tableConfig, exists := config.GetTableConfig("test-table")
			Expect(exists).To(BeTrue())
			Expect(tableConfig.TableName).To(Equal("test-dynamodb-table"))
		})

		It("should return false when the table config does not exist", func() {
			config := &Config{
				Tables: map[string]TableConfig{
					"test-table": {TableName: "test-dynamodb-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				},
			}
			_, exists := config.GetTableConfig("non-existent")
			Expect(exists).To(BeFalse())
		})
	})

	Describe("Validate", func() {
		DescribeTable("validation cases",
			func(config *Config, expectedErrors int, errorFields []string) {
				errors := config.Validate()
				Expect(errors).To(HaveLen(expectedErrors))
				if expectedErrors > 0 {
					errorFieldsFound := make([]string, len(errors))
					for i, err := range errors {
						errorFieldsFound[i] = err.Field
					}
					for _, expectedField := range errorFields {
						Expect(errorFieldsFound).To(ContainElement(expectedField))
					}
				}
			},
			Entry("valid config", &Config{Tables: map[string]TableConfig{"test-table": {TableName: "test-dynamodb-table", Endpoint: "http://localhost:8000", Region: "us-east-1"}}}, 0, []string(nil)),
			Entry("missing table_name", &Config{Tables: map[string]TableConfig{"test-table": {Endpoint: "http://localhost:8000", Region: "us-east-1"}}}, 1, []string{"tables.test-table.table_name"}),
			Entry("missing endpoint", &Config{Tables: map[string]TableConfig{"test-table": {TableName: "test-dynamodb-table", Region: "us-east-1"}}}, 1, []string{"tables.test-table.endpoint"}),
			Entry("missing region", &Config{Tables: map[string]TableConfig{"test-table": {TableName: "test-dynamodb-table", Endpoint: "http://localhost:8000"}}}, 1, []string{"tables.test-table.region"}),
			Entry("multiple missing fields", &Config{Tables: map[string]TableConfig{"test-table": {}}}, 3, []string{"tables.test-table.table_name", "tables.test-table.endpoint", "tables.test-table.region"}),
		)
	})

	Describe("ApplyDefaults", func() {
		DescribeTable("default application cases",
			func(input TableConfig, expected TableConfig) {
				config := input
				config.ApplyDefaults()
				Expect(config).To(Equal(expected))
			},
			Entry("empty config", TableConfig{}, TableConfig{Endpoint: DefaultEndpoint, Region: DefaultRegion, AccessKeyID: DefaultAccessKeyID, SecretAccessKey: DefaultSecretAccessKey}),
			Entry("partial config", TableConfig{TableName: "test-table", Endpoint: "http://custom:8000"}, TableConfig{TableName: "test-table", Endpoint: "http://custom:8000", Region: DefaultRegion, AccessKeyID: DefaultAccessKeyID, SecretAccessKey: DefaultSecretAccessKey}),
			Entry("complete config", TableConfig{TableName: "test-table", Endpoint: "http://custom:8000", Region: "us-west-2", AccessKeyID: "custom-key", SecretAccessKey: "custom-secret"}, TableConfig{TableName: "test-table", Endpoint: "http://custom:8000", Region: "us-west-2", AccessKeyID: "custom-key", SecretAccessKey: "custom-secret"}),
		)
	})

	Describe("MergeWithFlags", func() {
		var baseConfig TableConfig
		BeforeEach(func() {
			baseConfig = TableConfig{TableName: "base-table", Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "base-key", SecretAccessKey: "base-secret"}
		})

		DescribeTable("merge cases",
			func(flags map[string]interface{}, expected *MergedTableConfig) {
				result := baseConfig.MergeWithFlags(flags)
				Expect(result).To(Equal(expected))
			},
			Entry("no flags", map[string]interface{}{}, &MergedTableConfig{TableName: "base-table", Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "base-key", SecretAccessKey: "base-secret"}),
			Entry("override table name", map[string]interface{}{"table": "override-table"}, &MergedTableConfig{TableName: "override-table", Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "base-key", SecretAccessKey: "base-secret"}),
			Entry("override multiple fields", map[string]interface{}{"endpoint": "http://override:9000", "region": "us-west-2", "access_key": "override-key"}, &MergedTableConfig{TableName: "base-table", Endpoint: "http://override:9000", Region: "us-west-2", AccessKeyID: "override-key", SecretAccessKey: "base-secret"}),
			Entry("empty string flags should not override", map[string]interface{}{"table": "", "endpoint": "", "region": ""}, &MergedTableConfig{TableName: "base-table", Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "base-key", SecretAccessKey: "base-secret"}),
			Entry("non-string flags should not override", map[string]interface{}{"table": 123, "endpoint": true}, &MergedTableConfig{TableName: "base-table", Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "base-key", SecretAccessKey: "base-secret"}),
		)
	})

	Describe("CreateFallbackConfig", func() {
		DescribeTable("fallback cases",
			func(flags map[string]interface{}, expected *MergedTableConfig) {
				result := CreateFallbackConfig(flags)
				Expect(result).To(Equal(expected))
			},
			Entry("no flags - all defaults", map[string]interface{}{}, &MergedTableConfig{TableName: "", Endpoint: DefaultEndpoint, Region: DefaultRegion, AccessKeyID: DefaultAccessKeyID, SecretAccessKey: DefaultSecretAccessKey}),
			Entry("with table flag", map[string]interface{}{"table": "test-table"}, &MergedTableConfig{TableName: "test-table", Endpoint: DefaultEndpoint, Region: DefaultRegion, AccessKeyID: DefaultAccessKeyID, SecretAccessKey: DefaultSecretAccessKey}),
			Entry("override all with flags", map[string]interface{}{"table": "flag-table", "endpoint": "http://flag:9000", "region": "eu-west-1", "access_key": "flag-key", "secret_key": "flag-secret"}, &MergedTableConfig{TableName: "flag-table", Endpoint: "http://flag:9000", Region: "eu-west-1", AccessKeyID: "flag-key", SecretAccessKey: "flag-secret"}),
			Entry("empty string flags should not override defaults", map[string]interface{}{"endpoint": "", "region": ""}, &MergedTableConfig{TableName: "", Endpoint: DefaultEndpoint, Region: DefaultRegion, AccessKeyID: DefaultAccessKeyID, SecretAccessKey: DefaultSecretAccessKey}),
		)
	})

	Describe("ValidateMergedConfig", func() {
		DescribeTable("merged config validation cases",
			func(config *MergedTableConfig, expectError bool, errorMsg string) {
				err := config.Validate()
				if expectError {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorMsg))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("valid config", &MergedTableConfig{TableName: "test-table", Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "key", SecretAccessKey: "secret"}, false, ""),
			Entry("missing table name", &MergedTableConfig{Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "key", SecretAccessKey: "secret"}, true, "table name is required"),
			Entry("missing endpoint", &MergedTableConfig{TableName: "test-table", Region: "us-east-1", AccessKeyID: "key", SecretAccessKey: "secret"}, true, "endpoint is required"),
			Entry("missing region", &MergedTableConfig{TableName: "test-table", Endpoint: "http://localhost:8000", AccessKeyID: "key", SecretAccessKey: "secret"}, true, "region is required"),
		)
	})

	Describe("ResolveTableConfig", func() {
		var config *Config
		BeforeEach(func() {
			viper.Reset()
			config = &Config{
				Tables: map[string]TableConfig{
					"test-table": {TableName: "test-dynamodb-table", Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "config-key", SecretAccessKey: "config-secret"},
				},
			}
		})

		AfterEach(func() {
			FlagChangedFunc = nil
		})

		DescribeTable("resolve cases",
			func(tableConfigName string, viperFlags map[string]string, expectedConfig *MergedTableConfig, expectError bool, errorMsg string) {
				viper.Reset()
				for key, value := range viperFlags {
					viper.Set(key, value)
				}
				// Register FlagChangedFunc so GetViperFlags picks up the viper values.
				// Map viper keys to the CLI flag names that GetViperFlags checks.
				viperToFlag := map[string]string{
					"table":      "table",
					"endpoint":   "endpoint",
					"region":     "region",
					"access_key": "access-key",
					"secret_key": "secret-key",
				}
				FlagChangedFunc = func(name string) bool {
					for vk := range viperFlags {
						if flagName, ok := viperToFlag[vk]; ok && flagName == name {
							return true
						}
					}
					return false
				}
				result, err := config.ResolveTableConfig(tableConfigName)
				if expectError {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorMsg))
					Expect(result).To(BeNil())
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(expectedConfig))
				}
			},
			Entry("resolve existing table config", "test-table", map[string]string{}, &MergedTableConfig{TableName: "test-dynamodb-table", Endpoint: "http://localhost:8000", Region: "us-east-1", AccessKeyID: "config-key", SecretAccessKey: "config-secret"}, false, ""),
			Entry("resolve with flag override", "test-table", map[string]string{"endpoint": "http://override:9000", "region": "us-west-2"}, &MergedTableConfig{TableName: "test-dynamodb-table", Endpoint: "http://override:9000", Region: "us-west-2", AccessKeyID: "config-key", SecretAccessKey: "config-secret"}, false, ""),
			Entry("non-existent table config", "non-existent", map[string]string{}, (*MergedTableConfig)(nil), true, "Table configuration 'non-existent' not found"),
			Entry("fallback to CLI flags", "", map[string]string{"table": "flag-table", "endpoint": "http://flag:8000", "region": "eu-west-1"}, &MergedTableConfig{TableName: "flag-table", Endpoint: "http://flag:8000", Region: "eu-west-1", AccessKeyID: DefaultAccessKeyID, SecretAccessKey: DefaultSecretAccessKey}, false, ""),
			Entry("fallback with missing table name", "", map[string]string{"endpoint": "http://flag:8000", "region": "eu-west-1"}, (*MergedTableConfig)(nil), true, "Configuration validation failed"),
		)
	})

	Describe("GetDefaultConfigPath", func() {
		It("should return a path containing the expected config location", func() {
			path, err := GetDefaultConfigPath()
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(ContainSubstring(".config/querydb/config.yaml"))
		})
	})

	Describe("ValidationError", func() {
		It("should format the error message correctly", func() {
			err := ValidationError{Field: "test.field", Message: "test message"}
			Expect(err.Error()).To(Equal("validation error for field 'test.field': test message"))
		})
	})
})
