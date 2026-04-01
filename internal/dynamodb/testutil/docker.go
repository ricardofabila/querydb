package testutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

const (
	// DynamoDBLocalEndpoint is the endpoint for DynamoDB Local
	DynamoDBLocalEndpoint = "http://localhost:8000"
	
	// DynamoDBLocalRegion is the region for DynamoDB Local
	DynamoDBLocalRegion = "us-west-2"
	
	// DynamoDBLocalAccessKey is the dummy access key for DynamoDB Local
	DynamoDBLocalAccessKey = "foo"
	
	// DynamoDBLocalSecretKey is the dummy secret key for DynamoDB Local
	DynamoDBLocalSecretKey = "bar"
	
	// DynamoDBLocalPort is the port DynamoDB Local listens on
	DynamoDBLocalPort = 8000
	
	// ContainerName is the name of the DynamoDB Local container
	ContainerName = "dynamodb-local-test"
)

// StartDynamoDBLocal starts the DynamoDB Local Docker container
// Returns an error if the container fails to start or is not ready
func StartDynamoDBLocal(ctx context.Context) error {
	// Check if our test container is already running
	checkCmd := exec.CommandContext(ctx, "docker", "ps", "-q", "-f", fmt.Sprintf("name=%s", ContainerName))
	output, err := checkCmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		// Container is already running, check if it's ready
		if err := waitForDynamoDBReady(ctx, 5*time.Second); err == nil {
			return nil
		}
		// If not ready, stop and restart
		_ = CleanupDynamoDBLocal(ctx)
	}
	
	// Check if port 8000 is already in use by another container
	if isPortInUse(DynamoDBLocalPort) {
		// Check if it's a different DynamoDB container
		checkOtherCmd := exec.CommandContext(ctx, "docker", "ps", "-q", "-f", "publish=8000")
		otherOutput, err := checkOtherCmd.Output()
		if err == nil && len(strings.TrimSpace(string(otherOutput))) > 0 {
			// Another container is using port 8000, try to use it if it's DynamoDB
			if IsDynamoDBReady(ctx) {
				// Port 8000 has a working DynamoDB instance, use it
				return nil
			}
			return fmt.Errorf("port %d is already in use by another container, but DynamoDB is not responding", DynamoDBLocalPort)
		}
	}
	
	// Find workspace root (where docker-compose.test.yml is located)
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}
	
	// Start container using docker-compose
	cmd := exec.CommandContext(ctx, "docker-compose", "-f", "docker-compose.test.yml", "up", "-d")
	cmd.Dir = workspaceRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start DynamoDB Local container: %w\nOutput: %s", err, string(output))
	}
	
	// Wait for DynamoDB Local to be ready
	if err := waitForDynamoDBReady(ctx, 30*time.Second); err != nil {
		// Clean up on failure
		_ = CleanupDynamoDBLocal(ctx)
		return fmt.Errorf("DynamoDB Local did not become ready: %w", err)
	}
	
	return nil
}

// CleanupDynamoDBLocal stops and removes the DynamoDB Local Docker container
func CleanupDynamoDBLocal(ctx context.Context) error {
	// Find workspace root (where docker-compose.test.yml is located)
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}
	
	cmd := exec.CommandContext(ctx, "docker-compose", "-f", "docker-compose.test.yml", "down")
	cmd.Dir = workspaceRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop DynamoDB Local container: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// SeedTestData runs the seed script to populate DynamoDB Local with test data
func SeedTestData(ctx context.Context) error {
	// Find workspace root (where test/seed_data.sh is located)
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}
	
	// Use bash to run the seed script
	cmd := exec.CommandContext(ctx, "bash", "./test/seed_data.sh")
	cmd.Dir = workspaceRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to seed test data: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// CleanTestData removes all test tables from DynamoDB Local
func CleanTestData(ctx context.Context) error {
	// Find workspace root (where test/seed_data.sh is located)
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}
	
	// Use bash to run the seed script with clean argument
	cmd := exec.CommandContext(ctx, "bash", "./test/seed_data.sh", "clean")
	cmd.Dir = workspaceRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clean test data: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// IsDynamoDBReady checks if DynamoDB Local is ready to accept connections
func IsDynamoDBReady(ctx context.Context) bool {
	// Check if port is open
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", DynamoDBLocalPort), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	
	// Try to list tables to verify DynamoDB is responding
	config := &aws.Config{
		Region:      aws.String(DynamoDBLocalRegion),
		Endpoint:    aws.String(DynamoDBLocalEndpoint),
		Credentials: credentials.NewStaticCredentials(DynamoDBLocalAccessKey, DynamoDBLocalSecretKey, ""),
	}
	
	sess, err := session.NewSession(config)
	if err != nil {
		return false
	}
	
	client := dynamodb.New(sess)
	
	// Create a context with timeout for the list tables operation
	listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	_, err = client.ListTablesWithContext(listCtx, &dynamodb.ListTablesInput{})
	return err == nil
}

// waitForDynamoDBReady waits for DynamoDB Local to be ready with a timeout
func waitForDynamoDBReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if IsDynamoDBReady(ctx) {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	
	return fmt.Errorf("timeout waiting for DynamoDB Local to be ready after %v", timeout)
}

// findWorkspaceRoot finds the workspace root directory by looking for go.mod
func findWorkspaceRoot() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	
	// Walk up the directory tree looking for go.mod
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}
		
		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", fmt.Errorf("could not find workspace root (go.mod not found)")
		}
		dir = parent
	}
}

// isPortInUse checks if a TCP port is in use
func isPortInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

