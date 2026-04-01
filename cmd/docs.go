package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Show detailed help, configuration guide, and troubleshooting",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(docsText)
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}

const docsText = `
QueryDB — Detailed Documentation
=================================

  Repo: https://github.com/ricardofabila/querydb

QUICK START:

  1. Run 'querydb' to create a sample configuration file
  2. Edit ~/.config/querydb/config.yaml to add your table configurations
  3. Query tables with 'querydb table <config-name>'

USAGE PATTERNS:

  querydb table <config-name>                        Query using saved config
  querydb table --table <name> --endpoint <url>      Ad-hoc query with flags
  querydb table <config-name> --region us-west-2     Mix config + flag overrides
  querydb serve                                      Start the web UI
  querydb serve --port 9090 --host 0.0.0.0           Custom host/port

CONFIGURATION FILE:

  Location: ~/.config/querydb/config.yaml
  Created automatically on first run with sample entries.

  Structure:

    tables:
      <config-name>:
        table_name: "<actual-dynamodb-table-name>"
        endpoint: "<dynamodb-endpoint-url>"
        region: "<aws-region>"
        access_key_id: "<aws-access-key>"        # Optional
        secret_access_key: "<aws-secret-key>"    # Optional

  Example:

    tables:
      users:
        table_name: "Users"
        endpoint: "http://localhost:8000"
        region: "us-west-2"

DEFAULT VALUES:

  endpoint:    http://localhost:8000
  region:      us-east-1
  access_key:  foo
  secret_key:  bar
  output:      json

TROUBLESHOOTING:

  Connection Issues:
    • Ensure your DynamoDB instance is running at the configured endpoint
    • Check that the endpoint URL is correct and accessible

  Authentication Issues:
    • Ensure proper credentials are configured via:
      - Config file (access_key_id / secret_access_key)
      - CLI flags (--access-key / --secret-key)
      - Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
      - AWS credentials file (~/.aws/credentials)

  Table Not Found:
    • Verify the table name is correct and exists
    • Check that you're connecting to the right endpoint and region
    • Ensure you have permissions to access the table

  Configuration Issues:
    • Check YAML syntax in your configuration file
    • Ensure required fields are present (table_name, endpoint, region)

For more information, visit: https://github.com/ricardofabila/querydb
`
