# DynamoDB Test Utilities

This package provides helper functions for managing DynamoDB Local in Docker for integration testing.

## Overview

The testutil package simplifies integration testing with DynamoDB by providing:
- Docker container lifecycle management for DynamoDB Local
- Test data seeding and cleanup
- Readiness checks
- Reusable test constants

## Prerequisites

- Docker and docker-compose installed
- Fish shell (for running seed scripts)
- AWS CLI (for seeding data)
- jq (for parsing JSON in seed scripts)

## Quick Start

```go
import (
    "context"
    "testing"
    "time"
    
    "querydb/internal/dynamodb"
    "querydb/internal/dynamodb/testutil"
)

func TestMyFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()
    
    // Start DynamoDB Local
    if err := testutil.StartDynamoDBLocal(ctx); err != nil {
        t.Fatalf("Failed to start DynamoDB Local: %v", err)
    }
    
    // Seed test data (optional)
    if err := testutil.SeedTestData(ctx); err != nil {
        if !testutil.IsDynamoDBReady(ctx) {
            t.Fatalf("Failed to seed test data: %v", err)
        }
    }
    
    // Create client
    client, err := dynamodb.NewClient(
        testutil.DynamoDBLocalEndpoint,
        testutil.DynamoDBLocalRegion,
        testutil.DynamoDBLocalAccessKey,
        testutil.DynamoDBLocalSecretKey,
    )
    if err != nil {
        t.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()
    
    // Test your code...
}
```

## Functions

### StartDynamoDBLocal(ctx context.Context) error

Starts the DynamoDB Local Docker container using docker-compose.

- Checks if the container is already running and reuses it if ready
- If port 8000 is in use by another DynamoDB instance, reuses that
- Waits up to 30 seconds for DynamoDB to be ready
- Returns an error if the container fails to start or doesn't become ready

**Example:**
```go
if err := testutil.StartDynamoDBLocal(ctx); err != nil {
    t.Fatalf("Failed to start DynamoDB Local: %v", err)
}
```

### CleanupDynamoDBLocal(ctx context.Context) error

Stops and removes the DynamoDB Local Docker container.

**Note:** It's often better to leave the container running between tests for faster execution. Only clean up when necessary.

**Example:**
```go
defer testutil.CleanupDynamoDBLocal(ctx)
```

### SeedTestData(ctx context.Context) error

Runs the seed script (`test/seed_data.sh`) to populate DynamoDB Local with test data.

Creates the following tables:
- **Users**: Simple user data with nested profiles
- **Products**: Products with composite keys (productId, category)
- **Orders**: Orders with nested items and addresses
- **EdgeCases**: Various edge cases (empty strings, null values, deeply nested data)

**Note:** The seed script may produce fish shell warnings about `echo -e` but still succeeds. Check `IsDynamoDBReady()` to verify success.

**Example:**
```go
if err := testutil.SeedTestData(ctx); err != nil {
    if !testutil.IsDynamoDBReady(ctx) {
        t.Fatalf("Failed to seed test data: %v", err)
    }
    t.Logf("Seed script produced warnings but succeeded")
}
```

### CleanTestData(ctx context.Context) error

Removes all test tables from DynamoDB Local by running the seed script with the `clean` argument.

**Example:**
```go
if err := testutil.CleanTestData(ctx); err != nil {
    t.Logf("Failed to clean test data: %v", err)
}
```

### IsDynamoDBReady(ctx context.Context) bool

Checks if DynamoDB Local is ready to accept connections.

- Verifies port 8000 is open
- Attempts to list tables to confirm DynamoDB is responding
- Returns true if ready, false otherwise

**Example:**
```go
if !testutil.IsDynamoDBReady(ctx) {
    t.Fatal("DynamoDB Local is not ready")
}
```

## Constants

```go
const (
    DynamoDBLocalEndpoint   = "http://localhost:8000"
    DynamoDBLocalRegion     = "us-west-2"
    DynamoDBLocalAccessKey  = "foo"
    DynamoDBLocalSecretKey  = "bar"
    DynamoDBLocalPort       = 8000
    ContainerName           = "dynamodb-local-test"
)
```

## Test Data Structure

The seed data includes:

### Users Table
- Hash key: `userId` (String)
- 3 items with various data types (strings, numbers, booleans, nested maps, lists)

### Products Table
- Hash key: `productId` (String)
- Range key: `category` (String)
- 2 items with product details, specs, and reviews

### Orders Table
- Hash key: `orderId` (String)
- 2 items with order details, items, and shipping addresses

### EdgeCases Table
- Hash key: `id` (String)
- 3 items testing edge cases:
  - Empty strings, null values, zero/negative numbers
  - Deeply nested structures (5 levels)
  - Unicode strings, special characters, long strings

## Running Tests

```bash
# Run all tests (includes integration tests)
go test ./internal/dynamodb/testutil/

# Run only unit tests (skip integration tests)
go test -short ./internal/dynamodb/testutil/

# Run with verbose output
go test -v ./internal/dynamodb/testutil/

# Run specific test
go test -v ./internal/dynamodb/testutil/ -run TestExampleUsage
```

## Best Practices

1. **Reuse containers**: Don't clean up the container between tests unless necessary. This speeds up test execution.

2. **Use context timeouts**: Always use a context with timeout to prevent tests from hanging.

3. **Check readiness**: After seeding, verify DynamoDB is ready before proceeding.

4. **Skip in short mode**: Use `testing.Short()` to skip integration tests when running quick checks.

5. **Handle seed warnings**: The seed script may produce fish shell warnings but still succeed. Check `IsDynamoDBReady()` to verify.

## Troubleshooting

### Port 8000 already in use

If another DynamoDB instance is running on port 8000, `StartDynamoDBLocal()` will attempt to reuse it. If it's not responding, you'll need to stop the other instance:

```bash
docker ps | grep dynamodb
docker stop <container-id>
```

### Seed script errors

The seed script uses fish shell and may produce warnings about `echo -e` and `$argv`. These are cosmetic and don't affect functionality. The script still creates tables and loads data successfully.

### Container won't start

Check Docker logs:
```bash
docker logs dynamodb-local-test
```

Ensure docker-compose.test.yml exists in the workspace root.

### Tests hang

Ensure you're using a context with timeout:
```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()
```

## Manual Testing

You can also use these utilities for manual testing:

```bash
# Start DynamoDB Local
docker-compose -f docker-compose.test.yml up -d

# Seed test data
fish ./test/seed_data.sh

# Query tables
aws dynamodb scan --table-name Users \
    --endpoint-url http://localhost:8000 \
    --region us-west-2

# Clean up
fish ./test/seed_data.sh clean
docker-compose -f docker-compose.test.yml down
```
