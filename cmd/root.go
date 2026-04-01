package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"querydb/internal/config"
	"querydb/internal/dynamodb"
	"querydb/internal/errors"
	"querydb/internal/formatter"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "querydb",
	Short: "A CLI tool for querying DynamoDB tables locally",
	Long: `QueryDB — query DynamoDB tables locally to aid developer experience.

  querydb table <config-name>       Query a table by config name
  querydb serve                     Start the web UI
  querydb docs                      Show detailed help and troubleshooting

Config: ~/.config/querydb/config.yaml
Repo:   https://github.com/ricardofabila/querydb`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		handleError(err)
		os.Exit(1)
	}
}

// handleError provides user-friendly error display
func handleError(err error) {
	if queryErr, ok := err.(*errors.QueryError); ok {
		// Display structured error with formatting
		fmt.Fprintf(os.Stderr, "\n❌ Error: %s\n", queryErr.Message)
		
		if queryErr.Details != "" {
			fmt.Fprintf(os.Stderr, "\nDetails: %s\n", queryErr.Details)
		}
		
		if len(queryErr.Suggestions) > 0 {
			fmt.Fprintf(os.Stderr, "\n💡 Suggestions:\n")
			for _, suggestion := range queryErr.Suggestions {
				fmt.Fprintf(os.Stderr, "  • %s\n", suggestion)
			}
		}
		
		// Show underlying error for debugging if needed
		if queryErr.Cause != nil {
			fmt.Fprintf(os.Stderr, "\n🔍 Technical details: %v\n", queryErr.Cause)
		}
	} else {
		// Fallback for non-structured errors
		fmt.Fprintf(os.Stderr, "\n❌ Error: %v\n", err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Let the config package know which flags were explicitly set on the CLI,
	// so viper defaults don't silently override config-file values.
	config.FlagChangedFunc = func(name string) bool {
		return rootCmd.PersistentFlags().Lookup(name) != nil &&
			rootCmd.PersistentFlags().Lookup(name).Changed
	}

	// Set version information
	rootCmd.Version = "1.0.0"
	rootCmd.SetVersionTemplate("QueryDB version {{.Version}}\n")

	// Global flags with detailed descriptions
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", 
		"Path to configuration file (default: $HOME/.config/querydb/config.yaml)")
	
	rootCmd.PersistentFlags().StringP("table", "t", "", 
		"DynamoDB table name (required if not using table configuration)")
	
	rootCmd.PersistentFlags().StringP("endpoint", "e", "http://localhost:8000", 
		"DynamoDB endpoint URL")
	
	rootCmd.PersistentFlags().StringP("region", "r", "us-east-1", 
		"AWS region")
	
	rootCmd.PersistentFlags().String("access-key", "", 
		"AWS access key ID")
	
	rootCmd.PersistentFlags().String("secret-key", "", 
		"AWS secret access key")
	
	rootCmd.PersistentFlags().StringP("output", "o", "json", 
		"Output format for query results (currently only 'json' is supported)")

	// Bind flags to viper
	viper.BindPFlag("table", rootCmd.PersistentFlags().Lookup("table"))
	viper.BindPFlag("endpoint", rootCmd.PersistentFlags().Lookup("endpoint"))
	viper.BindPFlag("region", rootCmd.PersistentFlags().Lookup("region"))
	viper.BindPFlag("access_key", rootCmd.PersistentFlags().Lookup("access-key"))
	viper.BindPFlag("secret_key", rootCmd.PersistentFlags().Lookup("secret-key"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	// Simplified usage template
	rootCmd.SetUsageTemplate(`Usage:{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	var configPath string
	
	if cfgFile != "" {
		// Use config file from the flag.
		configPath = cfgFile
		viper.SetConfigFile(cfgFile)
	} else {
		// Use default config path
		defaultPath, err := config.GetDefaultConfigPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting default config path: %v\n", err)
			os.Exit(1)
		}
		configPath = defaultPath
		viper.SetConfigFile(defaultPath)
	}

	// Set config type
	viper.SetConfigType("yaml")
	
	// Set default values from our config package
	viper.SetDefault("endpoint", config.DefaultEndpoint)
	viper.SetDefault("region", config.DefaultRegion)
	viper.SetDefault("access_key", config.DefaultAccessKeyID)
	viper.SetDefault("secret_key", config.DefaultSecretAccessKey)
	viper.SetDefault("output", config.DefaultOutputFormat)

	// Read in environment variables that match
	viper.AutomaticEnv()

	// Try to read the config file
	if err := viper.ReadInConfig(); err != nil {
		// If config file doesn't exist, try to create it with sample configuration
		if os.IsNotExist(err) {
			// Load config using our config package to trigger sample creation
			_, configErr := config.LoadConfig(configPath)
			if configErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not create sample config file: %v\n", configErr)
			} else {
				fmt.Fprintf(os.Stderr, "Created sample config file at: %s\n", configPath)
				// Try to read the newly created config
				if readErr := viper.ReadInConfig(); readErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: Could not read newly created config file: %v\n", readErr)
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Could not read config file: %v\n", err)
		}
	}
}

// runQuery executes the main query logic - shared between root and query commands
func runQuery(args []string) error {
	// Determine if we have a table config name argument
	var tableConfigName string
	if len(args) > 0 {
		tableConfigName = args[0]
	}

	// Load configuration file
	configPath := cfgFile
	if configPath == "" {
		defaultPath, err := config.GetDefaultConfigPath()
		if err != nil {
			return errors.NewConfigurationError(
				"Failed to get default config path",
				err,
				"Check if your home directory is accessible",
				"Ensure you have proper file system permissions",
			)
		}
		configPath = defaultPath
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// LoadConfig already returns structured errors
		return err
	}

	// Resolve table configuration (either from config file or CLI flags)
	tableConfig, err := cfg.ResolveTableConfig(tableConfigName)
	if err != nil {
		// ResolveTableConfig already returns structured errors
		return err
	}

	// Validate the resolved configuration
	if err := tableConfig.Validate(); err != nil {
		return errors.NewValidationError(
			"Final configuration validation failed",
			err,
			"Check that all required parameters are provided",
			"Use --help to see all available options",
		)
	}

	// Create DynamoDB client
	client, err := dynamodb.NewClient(
		tableConfig.Endpoint,
		tableConfig.Region,
		tableConfig.AccessKeyID,
		tableConfig.SecretAccessKey,
	)
	if err != nil {
		// NewClient already returns structured errors
		return err
	}
	defer client.Close()

	// Scan the table
	items, err := client.ScanTable(tableConfig.TableName)
	if err != nil {
		// ScanTable already returns structured errors
		return err
	}

	// Format and display results with summary
	output, err := formatter.FormatJSONWithSummary(items, tableConfig.TableName)
	if err != nil {
		return errors.NewFormatError(
			"Failed to format query results",
			err,
			"This might be due to complex data structures in the table",
			"Try querying a different table to isolate the issue",
			"Check if the table contains unsupported data types",
		)
	}

	// Print results to stdout
	fmt.Println(output)

	// Print additional summary to stderr for user feedback
	// fmt.Fprintf(os.Stderr, "\nQuery completed successfully. Found %d records.\n", len(items))

	return nil
}