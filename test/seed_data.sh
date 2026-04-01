#!/bin/bash

# seed_data.sh - Seed DynamoDB Local with test data
# This script creates test tables and loads data from seed_data.json
#
# Usage:
#   ./seed_data.sh          # Seed data (default)
#   ./seed_data.sh clean    # Delete all test tables

# Configuration
DYNAMODB_ENDPOINT="http://localhost:8000"
AWS_REGION="us-west-2"
export AWS_ACCESS_KEY_ID="foo"
export AWS_SECRET_ACCESS_KEY="bar"
export AWS_DEFAULT_REGION="$AWS_REGION"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SEED_DATA_FILE="$SCRIPT_DIR/seed_data.json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored messages
log_info() {
    printf "${GREEN}[INFO]${NC} %s\n" "$*"
}

log_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$*"
}

log_warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$*"
}

# Function to wait for DynamoDB Local to be ready
wait_for_dynamodb() {
    log_info "Waiting for DynamoDB Local to be ready..."
    local max_attempts=30
    local attempt=0

    while [ "$attempt" -lt "$max_attempts" ]; do
        if aws dynamodb list-tables \
            --endpoint-url "$DYNAMODB_ENDPOINT" \
            --region "$AWS_REGION" \
            --no-cli-pager \
            >/dev/null 2>&1; then
            log_info "DynamoDB Local is ready!"
            return 0
        fi

        attempt=$((attempt + 1))
        echo -n "."
        sleep 1
    done

    log_error "DynamoDB Local did not become ready after $max_attempts seconds"
    return 1
}

# Function to create a table
create_table() {
    local table_name="$1"
    local key_schema="$2"
    local attribute_definitions="$3"

    log_info "Creating table: $table_name"

    # Try to create the table
    local create_output
    create_output=$(aws dynamodb create-table \
        --table-name "$table_name" \
        --key-schema "$key_schema" \
        --attribute-definitions "$attribute_definitions" \
        --billing-mode PAY_PER_REQUEST \
        --endpoint-url "$DYNAMODB_ENDPOINT" \
        --region "$AWS_REGION" \
        --no-cli-pager 2>&1)

    local exit_code=$?

    if [ "$exit_code" -eq 0 ]; then
        log_info "Table $table_name created successfully"
        return 0
    elif echo "$create_output" | grep -q "ResourceInUseException"; then
        log_warn "Table $table_name already exists, skipping creation"
        return 0
    else
        log_error "Failed to create table $table_name: $create_output"
        return 1
    fi
}

# Function to put an item into a table
put_item() {
    local table_name="$1"
    local item="$2"

    if aws dynamodb put-item \
        --table-name "$table_name" \
        --item "$item" \
        --endpoint-url "$DYNAMODB_ENDPOINT" \
        --region "$AWS_REGION" \
        --no-cli-pager \
        >/dev/null 2>&1; then
        return 0
    else
        log_error "Failed to put item into $table_name"
        return 1
    fi
}

# Function to delete all test tables
clean_tables() {
    log_info "Cleaning up test tables..."

    # Check if seed data file exists
    if [ ! -f "$SEED_DATA_FILE" ]; then
        log_error "Seed data file not found: $SEED_DATA_FILE"
        exit 1
    fi

    # Parse seed data and delete tables
    local table_count
    table_count=$(jq '.tables | length' "$SEED_DATA_FILE")

    log_info "Found $table_count tables to delete"

    for ((i = 0; i < table_count; i++)); do
        local table_name
        table_name=$(jq -r ".tables[$i].tableName" "$SEED_DATA_FILE")

        log_info "Deleting table: $table_name"

        if aws dynamodb delete-table \
            --table-name "$table_name" \
            --endpoint-url "$DYNAMODB_ENDPOINT" \
            --region "$AWS_REGION" \
            --no-cli-pager \
            >/dev/null 2>&1; then
            log_info "Table $table_name deleted successfully"
        else
            log_warn "Table $table_name may not exist or deletion failed"
        fi
    done

    log_info "Cleanup completed!"
}

# Main execution
main() {
    # Check for clean command
    if [ "$#" -gt 0 ] && [ "$1" = "clean" ]; then
        # Wait for DynamoDB Local to be ready
        if ! wait_for_dynamodb; then
            exit 1
        fi

        clean_tables
        exit 0
    fi

    log_info "Starting DynamoDB Local seeding process..."

    # Check if seed data file exists
    if [ ! -f "$SEED_DATA_FILE" ]; then
        log_error "Seed data file not found: $SEED_DATA_FILE"
        exit 1
    fi

    # Check if jq is installed
    if ! command -v jq >/dev/null 2>&1; then
        log_error "jq is required but not installed. Please install jq to continue."
        exit 1
    fi

    # Check if AWS CLI is installed
    if ! command -v aws >/dev/null 2>&1; then
        log_error "AWS CLI is required but not installed. Please install AWS CLI to continue."
        exit 1
    fi

    # Wait for DynamoDB Local to be ready
    if ! wait_for_dynamodb; then
        exit 1
    fi

    # Parse seed data and create tables
    local table_count
    table_count=$(jq '.tables | length' "$SEED_DATA_FILE")

    log_info "Found $table_count tables to create"

    for ((i = 0; i < table_count; i++)); do
        # Extract table information
        local table_name key_schema attribute_definitions items
        table_name=$(jq -r ".tables[$i].tableName" "$SEED_DATA_FILE")
        key_schema=$(jq -c ".tables[$i].keySchema" "$SEED_DATA_FILE")
        attribute_definitions=$(jq -c ".tables[$i].attributeDefinitions" "$SEED_DATA_FILE")
        items=$(jq -c ".tables[$i].items" "$SEED_DATA_FILE")

        # Create table (continue even if it fails - table might already exist)
        create_table "$table_name" "$key_schema" "$attribute_definitions" || true

        # Load items
        local item_count
        item_count=$(echo "$items" | jq 'length')
        log_info "Loading $item_count items into $table_name"

        for ((j = 0; j < item_count; j++)); do
            local item
            item=$(echo "$items" | jq -c ".[$j]")

            if put_item "$table_name" "$item"; then
                echo -n "."
            else
                log_error "Failed to load item $((j + 1)) into $table_name"
            fi
        done

        echo "" # New line after dots
        log_info "Loaded $item_count items into $table_name"
    done

    log_info "Seeding completed successfully!"
    log_info "You can now query tables at $DYNAMODB_ENDPOINT"
}

# Run main function
main "$@"
