package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"querydb/internal/config"
	"querydb/internal/server"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server with web UI for DynamoDB management",
	Long: `Starts an HTTP server that serves a web-based user interface for browsing
and managing DynamoDB tables. The web UI provides functionality similar to the
AWS Console's DynamoDB Explorer, allowing you to view, create, update, and delete
items through a browser interface.

The server uses your existing table configurations from the config file, so all
configured tables are immediately available in the web UI.

EXAMPLES:

  Start with default settings (localhost:3030):
    querydb serve

  Start on a custom port:
    querydb serve --port 3000

  Bind to all interfaces:
    querydb serve --host 0.0.0.0

  Use a custom config file:
    querydb serve --config /path/to/config.yaml

  Combine options:
    querydb serve --host 0.0.0.0 --port 9090 --config ./my-config.yaml`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntP("port", "p", 3030, "Port to run the server on")
	serveCmd.Flags().StringP("host", "H", "127.0.0.1", "Host address to bind to")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Get flags
	port, _ := cmd.Flags().GetInt("port")
	host, _ := cmd.Flags().GetString("host")

	// Load configuration (same pattern as runQuery in root.go)
	configPath := cfgFile
	if configPath == "" {
		defaultPath, err := config.GetDefaultConfigPath()
		if err != nil {
			return fmt.Errorf("failed to get default config path: %w", err)
		}
		configPath = defaultPath
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create server
	srv := server.New(cfg, host, port)

	// Setup graceful shutdown via signal handling
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Graceful shutdown with 5-second timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	fmt.Println("Server stopped.")
	return nil
}
