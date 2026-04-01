package cmd

import (
	"github.com/spf13/cobra"
)

// tableCmd represents the table command
var tableCmd = &cobra.Command{
	Use:   "table [table-config-name] [flags]",
	Short: "Query a DynamoDB table using configuration or flags",
	Long: `Query a DynamoDB table using either a table configuration name from the config file
or command-line flags.

The table command scans the entire table and returns all records in JSON format.
It automatically handles DynamoDB pagination and converts DynamoDB attribute types
to readable formats.

EXAMPLES:

  Query with saved configuration:
    querydb table users              # Use users config
    querydb table products           # Use products config

  Query with flags (no configuration):
    querydb table --table Products --endpoint http://localhost:8000

  Override configuration settings:
    querydb table users --endpoint http://localhost:4566
    querydb table users --region us-west-1`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runQuery(args)
	},
}

func init() {
	rootCmd.AddCommand(tableCmd)
}
