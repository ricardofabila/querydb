package dynamodb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	
	"querydb/internal/errors"
)

// Client interface for DynamoDB operations
type Client interface {
	ScanTable(tableName string) ([]map[string]interface{}, error)
	PutItem(tableName string, item map[string]interface{}) error
	UpdateItem(tableName string, key map[string]interface{}, updates map[string]interface{}) error
	DeleteItem(tableName string, key map[string]interface{}) error
	GetItem(tableName string, key map[string]interface{}) (map[string]interface{}, error)
	Close() error

	DescribeTable(tableName string) (*TableMetadata, error)
	QueryTable(params QueryParams) (*QueryResult, error)
	ScanTableAdvanced(params ScanParams) (*QueryResult, error)
}

// TableMetadata holds DescribeTable results
type TableMetadata struct {
	TableName     string             `json:"table_name"`
	KeySchema     []KeySchemaElement `json:"key_schema"`
	AttributeDefs []AttributeDef     `json:"attribute_definitions"`
	GSIs          []IndexInfo        `json:"global_secondary_indexes,omitempty"`
	LSIs          []IndexInfo        `json:"local_secondary_indexes,omitempty"`
}

// KeySchemaElement represents a key attribute in a table or index key schema
type KeySchemaElement struct {
	AttributeName string `json:"attribute_name"`
	KeyType       string `json:"key_type"` // "HASH" or "RANGE"
}

// AttributeDef represents an attribute definition from DescribeTable
type AttributeDef struct {
	AttributeName string `json:"attribute_name"`
	AttributeType string `json:"attribute_type"` // "S", "N", "B"
}

// IndexInfo represents a Global or Local Secondary Index
type IndexInfo struct {
	IndexName  string             `json:"index_name"`
	KeySchema  []KeySchemaElement `json:"key_schema"`
	Projection string             `json:"projection"` // "ALL", "KEYS_ONLY", "INCLUDE"
}

// QueryParams holds parameters for QueryTable
type QueryParams struct {
	TableName         string
	IndexName         string // empty = base table
	KeyConditionExpr  string
	FilterExpr        string
	ProjectionExpr    string
	ExprAttrNames     map[string]string
	ExprAttrValues    map[string]*dynamodb.AttributeValue
	ExclusiveStartKey map[string]*dynamodb.AttributeValue
	ScanForward       bool
	Limit             *int64
}

// ScanParams holds parameters for ScanTableAdvanced
type ScanParams struct {
	TableName         string
	IndexName         string
	FilterExpr        string
	ProjectionExpr    string
	ExprAttrNames     map[string]string
	ExprAttrValues    map[string]*dynamodb.AttributeValue
	ExclusiveStartKey map[string]*dynamodb.AttributeValue
	Limit             *int64
}

// QueryResult holds the result of a Query or Scan operation
type QueryResult struct {
	Items            []map[string]*dynamodb.AttributeValue `json:"items"`
	Count            int64                                 `json:"count"`
	ScannedCount     int64                                 `json:"scanned_count"`
	LastEvaluatedKey map[string]*dynamodb.AttributeValue   `json:"last_evaluated_key,omitempty"`
	ConsumedCapacity *dynamodb.ConsumedCapacity            `json:"consumed_capacity,omitempty"`
}

// DynamoDBClient implements the Client interface
type DynamoDBClient struct {
	client  *dynamodb.DynamoDB
	session *session.Session
}

// NewClient creates a new DynamoDB client with AWS SDK configuration
func NewClient(endpoint, region, accessKey, secretKey string) (Client, error) {
	// Create AWS configuration
	config := &aws.Config{
		Region: aws.String(region),
	}

	// Set endpoint for LocalStack or custom DynamoDB endpoint
	if endpoint != "" {
		config.Endpoint = aws.String(endpoint)
	}

	// Set credentials if provided, otherwise use default credential chain
	if accessKey != "" && secretKey != "" {
		config.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}

	// Create AWS session
	sess, err := session.NewSession(config)
	if err != nil {
		return nil, errors.NewConnectionError(
			"Failed to create AWS session",
			err,
			errors.ConnectionSuggestions...,
		)
	}

	// Create DynamoDB client
	client := dynamodb.New(sess)

	return &DynamoDBClient{
		client:  client,
		session: sess,
	}, nil
}

// ScanTable scans a DynamoDB table and returns all items
func (c *DynamoDBClient) ScanTable(tableName string) ([]map[string]interface{}, error) {
	var items []map[string]interface{}
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue

	for {
		// Create scan input
		input := &dynamodb.ScanInput{
			TableName: aws.String(tableName),
		}

		// Set pagination token if available
		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		// Execute scan
		result, err := c.client.Scan(input)
		if err != nil {
			return nil, c.handleDynamoDBError(err, tableName)
		}

		// Convert DynamoDB items to Go maps
		for _, item := range result.Items {
			convertedItem, err := convertDynamoDBItem(item)
			if err != nil {
				return nil, errors.NewFormatError(
					"Failed to convert DynamoDB item to readable format",
					err,
					"This might be due to unsupported data types",
					"Check if the table contains complex nested structures",
					"Try querying a different table to isolate the issue",
				)
			}
			items = append(items, convertedItem)
		}

		// Check if there are more items to scan
		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return items, nil
}

// handleDynamoDBError converts AWS DynamoDB errors to structured QueryErrors
func (c *DynamoDBClient) handleDynamoDBError(err error, tableName string) error {
	if awsErr, ok := err.(awserr.Error); ok {
		switch awsErr.Code() {
		case dynamodb.ErrCodeResourceNotFoundException:
			return errors.NewQueryError(
				fmt.Sprintf("Table '%s' not found", tableName),
				err,
				errors.TableNotFoundSuggestions...,
			)
		case dynamodb.ErrCodeProvisionedThroughputExceededException:
			return errors.NewQueryError(
				"Table throughput exceeded",
				err,
				"Wait a moment and try again",
				"Consider increasing table throughput if this persists",
				"Check if other processes are heavily using the table",
			)
		case "UnrecognizedClientException":
			return errors.NewConnectionError(
				"Authentication failed - invalid credentials",
				err,
				errors.AuthSuggestions...,
			)
		case "NetworkingError", "RequestError":
			return errors.NewConnectionError(
				"Network error connecting to DynamoDB",
				err,
				errors.ConnectionSuggestions...,
			)
		default:
			// Check if it's a connection-related error based on message
			errMsg := strings.ToLower(awsErr.Message())
			if strings.Contains(errMsg, "connection") || 
			   strings.Contains(errMsg, "network") || 
			   strings.Contains(errMsg, "timeout") ||
			   strings.Contains(errMsg, "refused") {
				return errors.NewConnectionError(
					fmt.Sprintf("Connection error: %s", awsErr.Message()),
					err,
					errors.ConnectionSuggestions...,
				)
			}
			
			return errors.NewQueryError(
				fmt.Sprintf("DynamoDB operation failed: %s", awsErr.Message()),
				err,
				"Check the AWS error code and message for more details",
				"Verify your table configuration and permissions",
				fmt.Sprintf("AWS Error Code: %s", awsErr.Code()),
			)
		}
	}

	// Non-AWS error
	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "connection") || 
	   strings.Contains(errMsg, "network") || 
	   strings.Contains(errMsg, "dial") ||
	   strings.Contains(errMsg, "timeout") {
		return errors.NewConnectionError(
			"Connection error",
			err,
			errors.ConnectionSuggestions...,
		)
	}

	return errors.NewQueryError(
		"Unexpected error during table scan",
		err,
		"Check the error details for more information",
		"Verify your DynamoDB configuration",
	)
}

// PutItem creates or replaces an item in the table
func (c *DynamoDBClient) PutItem(tableName string, item map[string]interface{}) error {
	// Convert Go map to DynamoDB AttributeValue map
	attributeMap, err := convertToAttributeValueMap(item)
	if err != nil {
		return errors.NewValidationError(
			"Failed to convert item to DynamoDB format",
			err,
			"Check that all attribute values are valid DynamoDB types",
			"Ensure nested structures are properly formatted",
		)
	}

	// Create PutItem input
	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      attributeMap,
	}

	// Execute PutItem operation
	_, err = c.client.PutItem(input)
	if err != nil {
		return c.handleDynamoDBError(err, tableName)
	}

	return nil
}

// UpdateItem updates an existing item in the table
func (c *DynamoDBClient) UpdateItem(tableName string, key map[string]interface{}, updates map[string]interface{}) error {
	// Convert key to DynamoDB AttributeValue map
	keyAttributeMap, err := convertToAttributeValueMap(key)
	if err != nil {
		return errors.NewValidationError(
			"Failed to convert key to DynamoDB format",
			err,
			"Check that all key attributes are valid DynamoDB types",
			"Ensure key structure matches the table's key schema",
		)
	}

	// Build UpdateExpression and ExpressionAttributeValues
	updateExpression, expressionAttributeNames, expressionAttributeValues, err := buildUpdateExpression(updates)
	if err != nil {
		return errors.NewValidationError(
			"Failed to build update expression",
			err,
			"Check that all update values are valid DynamoDB types",
			"Ensure attribute names don't conflict with reserved words",
		)
	}

	// Create UpdateItem input
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(tableName),
		Key:                       keyAttributeMap,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
	}

	// Execute UpdateItem operation
	_, err = c.client.UpdateItem(input)
	if err != nil {
		return c.handleDynamoDBError(err, tableName)
	}

	return nil
}

// buildUpdateExpression builds an UpdateExpression from a map of updates
func buildUpdateExpression(updates map[string]interface{}) (string, map[string]*string, map[string]*dynamodb.AttributeValue, error) {
	if len(updates) == 0 {
		return "", nil, nil, fmt.Errorf("no updates provided")
	}

	var setParts []string
	expressionAttributeNames := make(map[string]*string)
	expressionAttributeValues := make(map[string]*dynamodb.AttributeValue)

	i := 0
	for attrName, attrValue := range updates {
		// Use expression attribute names to handle reserved words and special characters
		namePlaceholder := fmt.Sprintf("#attr%d", i)
		valuePlaceholder := fmt.Sprintf(":val%d", i)

		// Convert value to AttributeValue
		av, err := convertToAttributeValue(attrValue)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to convert attribute %s: %w", attrName, err)
		}

		expressionAttributeNames[namePlaceholder] = aws.String(attrName)
		expressionAttributeValues[valuePlaceholder] = av
		setParts = append(setParts, fmt.Sprintf("%s = %s", namePlaceholder, valuePlaceholder))

		i++
	}

	updateExpression := "SET " + strings.Join(setParts, ", ")
	return updateExpression, expressionAttributeNames, expressionAttributeValues, nil
}

// DeleteItem deletes an item from the table
func (c *DynamoDBClient) DeleteItem(tableName string, key map[string]interface{}) error {
	// Convert key to DynamoDB AttributeValue map
	keyAttributeMap, err := convertToAttributeValueMap(key)
	if err != nil {
		return errors.NewValidationError(
			"Failed to convert key to DynamoDB format",
			err,
			"Check that all key attributes are valid DynamoDB types",
			"Ensure key structure matches the table's key schema",
		)
	}

	// Create DeleteItem input
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       keyAttributeMap,
	}

	// Execute DeleteItem operation
	_, err = c.client.DeleteItem(input)
	if err != nil {
		return c.handleDynamoDBError(err, tableName)
	}

	return nil
}

// GetItem retrieves a single item by key
func (c *DynamoDBClient) GetItem(tableName string, key map[string]interface{}) (map[string]interface{}, error) {
	// Convert key to DynamoDB AttributeValue map
	keyAttributeMap, err := convertToAttributeValueMap(key)
	if err != nil {
		return nil, errors.NewValidationError(
			"Failed to convert key to DynamoDB format",
			err,
			"Check that all key attributes are valid DynamoDB types",
			"Ensure key structure matches the table's key schema",
		)
	}

	// Create GetItem input
	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       keyAttributeMap,
	}

	// Execute GetItem operation
	result, err := c.client.GetItem(input)
	if err != nil {
		return nil, c.handleDynamoDBError(err, tableName)
	}

	// Check if item was found
	if result.Item == nil {
		return nil, errors.NewQueryError(
			fmt.Sprintf("Item not found in table '%s'", tableName),
			nil,
			"Verify the key values are correct",
			"Check that the item exists in the table",
			"Ensure the key structure matches the table's key schema",
		)
	}

	// Convert DynamoDB item to Go map
	item, err := convertDynamoDBItem(result.Item)
	if err != nil {
		return nil, errors.NewFormatError(
			"Failed to convert DynamoDB item to readable format",
			err,
			"This might be due to unsupported data types",
			"Check if the item contains complex nested structures",
			"Try querying a different item to isolate the issue",
		)
	}

	return item, nil
}

// Close closes the DynamoDB client connection
func (c *DynamoDBClient) Close() error {
	// AWS SDK sessions don't require explicit closing
	// This method is provided for interface compatibility
	return nil
}

// DescribeTable retrieves table metadata including key schema, attribute definitions, and indexes.
func (c *DynamoDBClient) DescribeTable(tableName string) (*TableMetadata, error) {
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}
	result, err := c.client.DescribeTable(input)
	if err != nil {
		return nil, c.handleDynamoDBError(err, tableName)
	}
	meta := &TableMetadata{TableName: tableName}
	for _, ks := range result.Table.KeySchema {
		meta.KeySchema = append(meta.KeySchema, KeySchemaElement{
			AttributeName: *ks.AttributeName,
			KeyType:       *ks.KeyType,
		})
	}
	for _, ad := range result.Table.AttributeDefinitions {
		meta.AttributeDefs = append(meta.AttributeDefs, AttributeDef{
			AttributeName: *ad.AttributeName,
			AttributeType: *ad.AttributeType,
		})
	}
	for _, gsi := range result.Table.GlobalSecondaryIndexes {
		idx := IndexInfo{IndexName: *gsi.IndexName, Projection: *gsi.Projection.ProjectionType}
		for _, ks := range gsi.KeySchema {
			idx.KeySchema = append(idx.KeySchema, KeySchemaElement{
				AttributeName: *ks.AttributeName, KeyType: *ks.KeyType,
			})
		}
		meta.GSIs = append(meta.GSIs, idx)
	}
	for _, lsi := range result.Table.LocalSecondaryIndexes {
		idx := IndexInfo{IndexName: *lsi.IndexName, Projection: *lsi.Projection.ProjectionType}
		for _, ks := range lsi.KeySchema {
			idx.KeySchema = append(idx.KeySchema, KeySchemaElement{
				AttributeName: *ks.AttributeName, KeyType: *ks.KeyType,
			})
		}
		meta.LSIs = append(meta.LSIs, idx)
	}
	return meta, nil
}

// QueryTable executes a DynamoDB Query operation with the given parameters.
func (c *DynamoDBClient) QueryTable(params QueryParams) (*QueryResult, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(params.TableName),
		KeyConditionExpression: aws.String(params.KeyConditionExpr),
		ReturnConsumedCapacity: aws.String("TOTAL"),
	}
	if params.IndexName != "" {
		input.IndexName = aws.String(params.IndexName)
	}
	if params.FilterExpr != "" {
		input.FilterExpression = aws.String(params.FilterExpr)
	}
	if params.ProjectionExpr != "" {
		input.ProjectionExpression = aws.String(params.ProjectionExpr)
	}
	if len(params.ExprAttrNames) > 0 {
		names := make(map[string]*string)
		for k, v := range params.ExprAttrNames {
			names[k] = aws.String(v)
		}
		input.ExpressionAttributeNames = names
	}
	if len(params.ExprAttrValues) > 0 {
		input.ExpressionAttributeValues = params.ExprAttrValues
	}
	if params.ExclusiveStartKey != nil {
		input.ExclusiveStartKey = params.ExclusiveStartKey
	}
	if params.Limit != nil {
		input.Limit = params.Limit
	}
	input.ScanIndexForward = aws.Bool(params.ScanForward)

	result, err := c.client.Query(input)
	if err != nil {
		return nil, c.handleDynamoDBError(err, params.TableName)
	}
	return &QueryResult{
		Items:            result.Items,
		Count:            *result.Count,
		ScannedCount:     *result.ScannedCount,
		LastEvaluatedKey: result.LastEvaluatedKey,
		ConsumedCapacity: result.ConsumedCapacity,
	}, nil
}

// ScanTableAdvanced executes a DynamoDB Scan with optional filter, projection, and pagination.
func (c *DynamoDBClient) ScanTableAdvanced(params ScanParams) (*QueryResult, error) {
	input := &dynamodb.ScanInput{
		TableName:              aws.String(params.TableName),
		ReturnConsumedCapacity: aws.String("TOTAL"),
	}
	if params.IndexName != "" {
		input.IndexName = aws.String(params.IndexName)
	}
	if params.FilterExpr != "" {
		input.FilterExpression = aws.String(params.FilterExpr)
	}
	if params.ProjectionExpr != "" {
		input.ProjectionExpression = aws.String(params.ProjectionExpr)
	}
	if len(params.ExprAttrNames) > 0 {
		names := make(map[string]*string)
		for k, v := range params.ExprAttrNames {
			names[k] = aws.String(v)
		}
		input.ExpressionAttributeNames = names
	}
	if len(params.ExprAttrValues) > 0 {
		input.ExpressionAttributeValues = params.ExprAttrValues
	}
	if params.ExclusiveStartKey != nil {
		input.ExclusiveStartKey = params.ExclusiveStartKey
	}
	if params.Limit != nil {
		input.Limit = params.Limit
	}

	result, err := c.client.Scan(input)
	if err != nil {
		return nil, c.handleDynamoDBError(err, params.TableName)
	}
	return &QueryResult{
		Items:            result.Items,
		Count:            *result.Count,
		ScannedCount:     *result.ScannedCount,
		LastEvaluatedKey: result.LastEvaluatedKey,
		ConsumedCapacity: result.ConsumedCapacity,
	}, nil
}

// convertDynamoDBItem converts a DynamoDB item to a Go map
func convertDynamoDBItem(item map[string]*dynamodb.AttributeValue) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range item {
		convertedValue, err := convertAttributeValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert attribute %s: %w", key, err)
		}
		result[key] = convertedValue
	}

	return result, nil
}

// convertAttributeValue converts a DynamoDB AttributeValue to a Go interface{}
func convertAttributeValue(av *dynamodb.AttributeValue) (interface{}, error) {
	switch {
	case av.S != nil:
		// String type
		return *av.S, nil

	case av.N != nil:
		// Number type - try to parse as int first, then float
		if val, err := parseNumber(*av.N); err == nil {
			return val, nil
		}
		// If parsing fails, return as string
		return *av.N, nil

	case av.BOOL != nil:
		// Boolean type
		return *av.BOOL, nil

	case av.NULL != nil && *av.NULL:
		// Null type
		return nil, nil

	case av.L != nil:
		// List type
		var list []interface{}
		for _, item := range av.L {
			convertedItem, err := convertAttributeValue(item)
			if err != nil {
				return nil, err
			}
			list = append(list, convertedItem)
		}
		return list, nil

	case av.M != nil:
		// Map type
		result := make(map[string]interface{})
		for key, value := range av.M {
			convertedValue, err := convertAttributeValue(value)
			if err != nil {
				return nil, err
			}
			result[key] = convertedValue
		}
		return result, nil

	case av.SS != nil:
		// String Set type
		var stringSet []string
		for _, s := range av.SS {
			stringSet = append(stringSet, *s)
		}
		return stringSet, nil

	case av.NS != nil:
		// Number Set type
		var numberSet []interface{}
		for _, n := range av.NS {
			if val, err := parseNumber(*n); err == nil {
				numberSet = append(numberSet, val)
			} else {
				// If parsing fails, keep as string
				numberSet = append(numberSet, *n)
			}
		}
		return numberSet, nil

	case av.BS != nil:
		// Binary Set type - convert to base64 strings
		var binarySet []string
		for _, b := range av.BS {
			binarySet = append(binarySet, string(b))
		}
		return binarySet, nil

	case av.B != nil:
		// Binary type - convert to string
		return string(av.B), nil

	default:
		// Unknown or unsupported type - return as string representation
		return fmt.Sprintf("unsupported_type_%T", av), nil
	}
}

// parseNumber attempts to parse a DynamoDB number string as int64 or float64
func parseNumber(numStr string) (interface{}, error) {
	// Try to parse as integer first
	if intVal, err := strconv.ParseInt(numStr, 10, 64); err == nil {
		return intVal, nil
	}

	// If integer parsing fails, try float
	if floatVal, err := strconv.ParseFloat(numStr, 64); err == nil {
		return floatVal, nil
	}

	// If both fail, return error
	return nil, fmt.Errorf("unable to parse number: %s", numStr)
}

// convertToAttributeValueMap converts a Go map to a DynamoDB AttributeValue map
func convertToAttributeValueMap(item map[string]interface{}) (map[string]*dynamodb.AttributeValue, error) {
	result := make(map[string]*dynamodb.AttributeValue)

	for key, value := range item {
		av, err := convertToAttributeValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert attribute %s: %w", key, err)
		}
		result[key] = av
	}

	return result, nil
}

// convertToAttributeValue converts a Go interface{} to a DynamoDB AttributeValue
func convertToAttributeValue(value interface{}) (*dynamodb.AttributeValue, error) {
	if value == nil {
		return &dynamodb.AttributeValue{NULL: aws.Bool(true)}, nil
	}

	switch v := value.(type) {
	case string:
		return &dynamodb.AttributeValue{S: aws.String(v)}, nil

	case int:
		return &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(int64(v), 10))}, nil

	case int64:
		return &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(v, 10))}, nil

	case float64:
		return &dynamodb.AttributeValue{N: aws.String(strconv.FormatFloat(v, 'f', -1, 64))}, nil

	case bool:
		return &dynamodb.AttributeValue{BOOL: aws.Bool(v)}, nil

	case []interface{}:
		// List type
		list := make([]*dynamodb.AttributeValue, 0, len(v))
		for i, item := range v {
			av, err := convertToAttributeValue(item)
			if err != nil {
				return nil, fmt.Errorf("failed to convert list item at index %d: %w", i, err)
			}
			list = append(list, av)
		}
		return &dynamodb.AttributeValue{L: list}, nil

	case map[string]interface{}:
		// Map type
		m, err := convertToAttributeValueMap(v)
		if err != nil {
			return nil, err
		}
		return &dynamodb.AttributeValue{M: m}, nil

	case []string:
		// String Set type
		if len(v) == 0 {
			return nil, fmt.Errorf("string set cannot be empty")
		}
		ss := make([]*string, len(v))
		for i, s := range v {
			ss[i] = aws.String(s)
		}
		return &dynamodb.AttributeValue{SS: ss}, nil

	default:
		return nil, fmt.Errorf("unsupported type: %T", value)
	}
}