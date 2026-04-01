package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	sdkdynamodb "github.com/aws/aws-sdk-go/service/dynamodb"

	"querydb/internal/dynamodb"
	"querydb/internal/errors"
)

// APIResponse represents the standard API response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents an error in the API response
type APIError struct {
	Type        string   `json:"type"`
	Message     string   `json:"message"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// TableInfo represents information about a configured table
type TableInfo struct {
	ConfigName string `json:"config_name"`
	TableName  string `json:"table_name"`
	Endpoint   string `json:"endpoint"`
	Region     string `json:"region"`
}

// ItemsResponse represents the response for listing table items
type ItemsResponse struct {
	Items []map[string]interface{} `json:"items"`
	Count int                      `json:"count"`
}

// CreateItemRequest represents a request to create a new item
type CreateItemRequest struct {
	Item map[string]interface{} `json:"item"`
}

// UpdateItemRequest represents a request to update an existing item
type UpdateItemRequest struct {
	Updates map[string]interface{} `json:"updates"`
}

// QueryRequest is the JSON body for the query endpoint
type QueryRequest struct {
	IndexName         string                `json:"index_name,omitempty"`
	KeyConditionExpr  string                `json:"key_condition_expression"`
	FilterExpr        string                `json:"filter_expression,omitempty"`
	ProjectionExpr    string                `json:"projection_expression,omitempty"`
	ExprAttrNames     map[string]string     `json:"expression_attribute_names,omitempty"`
	ExprAttrValues    map[string]TypedValue `json:"expression_attribute_values,omitempty"`
	ExclusiveStartKey map[string]TypedValue `json:"exclusive_start_key,omitempty"`
	ScanForward       *bool                 `json:"scan_forward,omitempty"`
	Limit             *int64                `json:"limit,omitempty"`
}

// ScanRequest is the JSON body for the scan endpoint
type ScanRequest struct {
	IndexName         string                `json:"index_name,omitempty"`
	FilterExpr        string                `json:"filter_expression,omitempty"`
	ProjectionExpr    string                `json:"projection_expression,omitempty"`
	ExprAttrNames     map[string]string     `json:"expression_attribute_names,omitempty"`
	ExprAttrValues    map[string]TypedValue `json:"expression_attribute_values,omitempty"`
	ExclusiveStartKey map[string]TypedValue `json:"exclusive_start_key,omitempty"`
	Limit             *int64                `json:"limit,omitempty"`
}

// QueryScanResponse is the JSON response for query/scan endpoints
type QueryScanResponse struct {
	Items            []map[string]TypedValue `json:"items"`
	Count            int64                   `json:"count"`
	ScannedCount     int64                   `json:"scanned_count"`
	LastEvaluatedKey map[string]TypedValue   `json:"last_evaluated_key,omitempty"`
	ConsumedCapacity *ConsumedCapacityInfo   `json:"consumed_capacity,omitempty"`
}

// ConsumedCapacityInfo holds consumed capacity details
type ConsumedCapacityInfo struct {
	TableName     string  `json:"table_name"`
	CapacityUnits float64 `json:"capacity_units"`
}

// TypedValue represents a DynamoDB value with its type annotation
type TypedValue struct {
	Value interface{} `json:"value"`
	Type  string      `json:"type"` // S, N, B, BOOL, NULL, M, L, SS, NS, BS
}

// convertToTypedItem converts a raw DynamoDB item to typed format
func convertToTypedItem(item map[string]*sdkdynamodb.AttributeValue) map[string]TypedValue {
	result := make(map[string]TypedValue)
	for k, av := range item {
		result[k] = convertAVToTyped(av)
	}
	return result
}

// convertAVToTyped converts a single AttributeValue to TypedValue
func convertAVToTyped(av *sdkdynamodb.AttributeValue) TypedValue {
	switch {
	case av.S != nil:
		return TypedValue{Value: *av.S, Type: "S"}
	case av.N != nil:
		return TypedValue{Value: *av.N, Type: "N"}
	case av.B != nil:
		return TypedValue{Value: base64.StdEncoding.EncodeToString(av.B), Type: "B"}
	case av.BOOL != nil:
		return TypedValue{Value: *av.BOOL, Type: "BOOL"}
	case av.NULL != nil && *av.NULL:
		return TypedValue{Value: nil, Type: "NULL"}
	case av.M != nil:
		m := make(map[string]TypedValue)
		for k, v := range av.M {
			m[k] = convertAVToTyped(v)
		}
		return TypedValue{Value: m, Type: "M"}
	case av.L != nil:
		l := make([]TypedValue, len(av.L))
		for i, v := range av.L {
			l[i] = convertAVToTyped(v)
		}
		return TypedValue{Value: l, Type: "L"}
	case av.SS != nil:
		ss := make([]string, len(av.SS))
		for i, s := range av.SS {
			ss[i] = *s
		}
		return TypedValue{Value: ss, Type: "SS"}
	case av.NS != nil:
		ns := make([]string, len(av.NS))
		for i, n := range av.NS {
			ns[i] = *n
		}
		return TypedValue{Value: ns, Type: "NS"}
	case av.BS != nil:
		bs := make([]string, len(av.BS))
		for i, b := range av.BS {
			bs[i] = base64.StdEncoding.EncodeToString(b)
		}
		return TypedValue{Value: bs, Type: "BS"}
	default:
		return TypedValue{Value: nil, Type: "NULL"}
	}
}

// convertTypedToAV converts a TypedValue back to a DynamoDB AttributeValue.
// It handles JSON deserialization quirks where interface{} values may arrive
// as different Go types depending on how they were unmarshalled.
func convertTypedToAV(tv TypedValue) (*sdkdynamodb.AttributeValue, error) {
	switch tv.Type {
	case "S":
		s, ok := tv.Value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for type S")
		}
		return &sdkdynamodb.AttributeValue{S: aws.String(s)}, nil

	case "N":
		// N values are strings in DynamoDB wire format; after JSON round-trip they may be string or float64
		switch v := tv.Value.(type) {
		case string:
			return &sdkdynamodb.AttributeValue{N: aws.String(v)}, nil
		case float64:
			return &sdkdynamodb.AttributeValue{N: aws.String(fmt.Sprintf("%g", v))}, nil
		default:
			return nil, fmt.Errorf("expected string or number for type N, got %T", tv.Value)
		}

	case "BOOL":
		b, ok := tv.Value.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool for type BOOL")
		}
		return &sdkdynamodb.AttributeValue{BOOL: aws.Bool(b)}, nil

	case "NULL":
		return &sdkdynamodb.AttributeValue{NULL: aws.Bool(true)}, nil

	case "B":
		s, ok := tv.Value.(string)
		if !ok {
			return nil, fmt.Errorf("expected base64 string for type B")
		}
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 for type B: %w", err)
		}
		return &sdkdynamodb.AttributeValue{B: decoded}, nil

	case "M":
		// After JSON deserialization, M value arrives as map[string]interface{}
		rawMap, ok := tv.Value.(map[string]interface{})
		if !ok {
			// Could already be map[string]TypedValue from direct Go usage
			if typedMap, ok := tv.Value.(map[string]TypedValue); ok {
				m := make(map[string]*sdkdynamodb.AttributeValue)
				for k, v := range typedMap {
					av, err := convertTypedToAV(v)
					if err != nil {
						return nil, fmt.Errorf("error converting map key %q: %w", k, err)
					}
					m[k] = av
				}
				return &sdkdynamodb.AttributeValue{M: m}, nil
			}
			return nil, fmt.Errorf("expected map for type M, got %T", tv.Value)
		}
		m := make(map[string]*sdkdynamodb.AttributeValue)
		for k, v := range rawMap {
			childTV, err := interfaceToTypedValue(v)
			if err != nil {
				return nil, fmt.Errorf("error converting map key %q: %w", k, err)
			}
			av, err := convertTypedToAV(childTV)
			if err != nil {
				return nil, fmt.Errorf("error converting map key %q: %w", k, err)
			}
			m[k] = av
		}
		return &sdkdynamodb.AttributeValue{M: m}, nil

	case "L":
		// After JSON deserialization, L value arrives as []interface{}
		rawList, ok := tv.Value.([]interface{})
		if !ok {
			// Could already be []TypedValue from direct Go usage
			if typedList, ok := tv.Value.([]TypedValue); ok {
				l := make([]*sdkdynamodb.AttributeValue, len(typedList))
				for i, v := range typedList {
					av, err := convertTypedToAV(v)
					if err != nil {
						return nil, fmt.Errorf("error converting list index %d: %w", i, err)
					}
					l[i] = av
				}
				return &sdkdynamodb.AttributeValue{L: l}, nil
			}
			return nil, fmt.Errorf("expected list for type L, got %T", tv.Value)
		}
		l := make([]*sdkdynamodb.AttributeValue, len(rawList))
		for i, v := range rawList {
			childTV, err := interfaceToTypedValue(v)
			if err != nil {
				return nil, fmt.Errorf("error converting list index %d: %w", i, err)
			}
			av, err := convertTypedToAV(childTV)
			if err != nil {
				return nil, fmt.Errorf("error converting list index %d: %w", i, err)
			}
			l[i] = av
		}
		return &sdkdynamodb.AttributeValue{L: l}, nil

	case "SS":
		strs, err := toStringSlice(tv.Value)
		if err != nil {
			return nil, fmt.Errorf("expected string slice for type SS: %w", err)
		}
		ptrs := make([]*string, len(strs))
		for i, s := range strs {
			ptrs[i] = aws.String(s)
		}
		return &sdkdynamodb.AttributeValue{SS: ptrs}, nil

	case "NS":
		strs, err := toStringSlice(tv.Value)
		if err != nil {
			return nil, fmt.Errorf("expected string slice for type NS: %w", err)
		}
		ptrs := make([]*string, len(strs))
		for i, s := range strs {
			ptrs[i] = aws.String(s)
		}
		return &sdkdynamodb.AttributeValue{NS: ptrs}, nil

	case "BS":
		strs, err := toStringSlice(tv.Value)
		if err != nil {
			return nil, fmt.Errorf("expected string slice for type BS: %w", err)
		}
		bs := make([][]byte, len(strs))
		for i, s := range strs {
			decoded, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, fmt.Errorf("invalid base64 at index %d for type BS: %w", i, err)
			}
			bs[i] = decoded
		}
		return &sdkdynamodb.AttributeValue{BS: bs}, nil

	default:
		return nil, fmt.Errorf("unsupported type: %s", tv.Type)
	}
}

// interfaceToTypedValue converts a raw interface{} (from JSON deserialization)
// into a TypedValue. It expects a map with "value" and "type" keys.
func interfaceToTypedValue(v interface{}) (TypedValue, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return TypedValue{}, fmt.Errorf("expected object with value/type fields, got %T", v)
	}
	t, ok := m["type"].(string)
	if !ok {
		return TypedValue{}, fmt.Errorf("missing or invalid 'type' field")
	}
	return TypedValue{Value: m["value"], Type: t}, nil
}

// toStringSlice converts an interface{} to []string, handling both
// []string (direct Go) and []interface{} (JSON deserialized) cases.
func toStringSlice(v interface{}) ([]string, error) {
	switch val := v.(type) {
	case []string:
		return val, nil
	case []interface{}:
		result := make([]string, len(val))
		for i, item := range val {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("expected string at index %d, got %T", i, item)
			}
			result[i] = s
		}
		return result, nil
	default:
		return nil, fmt.Errorf("expected string slice, got %T", v)
	}
}

// sendSuccess sends a successful JSON response with the provided data
func (s *Server) sendSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

// sendError sends an error JSON response with the specified status code, message, and suggestions
func (s *Server) sendError(w http.ResponseWriter, status int, message string, suggestions []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error: &APIError{
			Type:        "error",
			Message:     message,
			Suggestions: suggestions,
		},
	})
}

// sendQueryError converts a QueryError to an HTTP response with appropriate status code
func (s *Server) sendQueryError(w http.ResponseWriter, err error) {
	if qErr, ok := err.(*errors.QueryError); ok {
		status := s.errorTypeToHTTPStatus(qErr.Type)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error: &APIError{
				Type:        string(qErr.Type),
				Message:     qErr.Message,
				Suggestions: qErr.Suggestions,
			},
		})
	} else {
		// Fallback for non-QueryError errors
		s.sendError(w, http.StatusInternalServerError, err.Error(), nil)
	}
}

// handleGetTables handles GET /api/tables - returns all configured tables
// Validates: Requirements 3.1, 3.2, 10.1
func (s *Server) handleGetTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	tables := make([]TableInfo, 0, len(s.config.Tables))
	for name, cfg := range s.config.Tables {
		tables = append(tables, TableInfo{
			ConfigName: name,
			TableName:  cfg.TableName,
			Endpoint:   cfg.Endpoint,
			Region:     cfg.Region,
		})
	}

	s.sendSuccess(w, tables)
}

// errorTypeToHTTPStatus maps QueryError types to appropriate HTTP status codes
func (s *Server) errorTypeToHTTPStatus(errType errors.ErrorType) int {
	switch errType {
	case errors.ErrorTypeValidation:
		return http.StatusBadRequest
	case errors.ErrorTypeConfiguration:
		return http.StatusBadRequest
	case errors.ErrorTypeConnection:
		return http.StatusServiceUnavailable
	case errors.ErrorTypeQuery:
		return http.StatusNotFound
	case errors.ErrorTypeFormat:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// handleTableOperations routes requests for /api/tables/{config-name}/{resource}[/{key}]
// Validates: Requirements 10.2, 10.3, 10.4, 10.5
func (s *Server) handleTableOperations(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/tables/{config-name}/{resource}[/{key}]
	path := strings.TrimPrefix(r.URL.Path, "/api/tables/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		s.sendError(w, http.StatusBadRequest, "Invalid path: expected /api/tables/{config-name}/{resource}", nil)
		return
	}

	configName := parts[0]
	resource := parts[1]

	switch resource {
	case "items":
		switch r.Method {
		case http.MethodGet:
			s.handleGetItems(w, r, configName)
		case http.MethodPost:
			s.handleCreateItem(w, r, configName)
		case http.MethodPut:
			if len(parts) < 3 || parts[2] == "" {
				s.sendError(w, http.StatusBadRequest, "Item key required for update", nil)
				return
			}
			s.handleUpdateItem(w, r, configName, parts[2])
		case http.MethodDelete:
			if len(parts) < 3 || parts[2] == "" {
				s.sendError(w, http.StatusBadRequest, "Item key required for delete", nil)
				return
			}
			s.handleDeleteItem(w, r, configName, parts[2])
		default:
			s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		}
	case "describe":
		if r.Method != http.MethodGet {
			s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
			return
		}
		s.handleDescribeTable(w, r, configName)
	case "query":
		if r.Method != http.MethodPost {
			s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
			return
		}
		s.handleQueryTable(w, r, configName)
	case "scan":
		if r.Method != http.MethodPost {
			s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
			return
		}
		s.handleScanTable(w, r, configName)
	default:
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Resource '%s' not found", resource), nil)
	}
}

// handleGetItems handles GET /api/tables/{config-name}/items
// Validates: Requirements 3.3, 4.1, 4.4, 4.5, 10.2
func (s *Server) handleGetItems(w http.ResponseWriter, r *http.Request, configName string) {
	tableConfig, exists := s.config.GetTableConfig(configName)
	if !exists {
		s.sendError(w, http.StatusNotFound,
			fmt.Sprintf("Table configuration '%s' not found", configName),
			[]string{"Check available table configurations"})
		return
	}

	tableConfig.ApplyDefaults()

	client, err := dynamodb.NewClient(
		tableConfig.Endpoint,
		tableConfig.Region,
		tableConfig.AccessKeyID,
		tableConfig.SecretAccessKey,
	)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}
	defer client.Close()

	items, err := client.ScanTable(tableConfig.TableName)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}

	if items == nil {
		items = []map[string]interface{}{}
	}

	s.sendSuccess(w, ItemsResponse{
		Items: items,
		Count: len(items),
	})
}

// handleCreateItem handles POST /api/tables/{config-name}/items
// Validates: Requirements 6.4, 6.5, 6.6, 10.3
func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request, configName string) {
	var req CreateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest,
			"Invalid request body",
			[]string{"Ensure request body is valid JSON", "Check item structure"})
		return
	}

	if len(req.Item) == 0 {
		s.sendError(w, http.StatusBadRequest,
			"Item data is empty",
			[]string{"Provide at least one attribute in the item"})
		return
	}

	tableConfig, exists := s.config.GetTableConfig(configName)
	if !exists {
		s.sendError(w, http.StatusNotFound,
			fmt.Sprintf("Table configuration '%s' not found", configName),
			[]string{"Check available table configurations"})
		return
	}

	tableConfig.ApplyDefaults()

	client, err := dynamodb.NewClient(
		tableConfig.Endpoint,
		tableConfig.Region,
		tableConfig.AccessKeyID,
		tableConfig.SecretAccessKey,
	)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}
	defer client.Close()

	if err := client.PutItem(tableConfig.TableName, req.Item); err != nil {
		s.sendQueryError(w, err)
		return
	}

	s.sendSuccess(w, map[string]string{"message": "Item created successfully"})
}

// handleUpdateItem handles PUT /api/tables/{config-name}/items/{key}
// Validates: Requirements 7.4, 7.5, 7.6, 10.4
func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request, configName string, encodedKey string) {
	// URL-decode the key
	decodedKey, err := url.PathUnescape(encodedKey)
	if err != nil {
		s.sendError(w, http.StatusBadRequest,
			"Invalid item key encoding",
			[]string{"Ensure the key is properly URL-encoded"})
		return
	}

	// Parse key from JSON
	var key map[string]interface{}
	if err := json.Unmarshal([]byte(decodedKey), &key); err != nil {
		s.sendError(w, http.StatusBadRequest,
			"Invalid item key format",
			[]string{"Key must be a JSON object", "Example: {\"id\": \"42\"}"})
		return
	}

	var req UpdateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest,
			"Invalid request body",
			[]string{"Ensure request body is valid JSON", "Check updates structure"})
		return
	}

	if len(req.Updates) == 0 {
		s.sendError(w, http.StatusBadRequest,
			"Updates data is empty",
			[]string{"Provide at least one attribute to update"})
		return
	}

	tableConfig, exists := s.config.GetTableConfig(configName)
	if !exists {
		s.sendError(w, http.StatusNotFound,
			fmt.Sprintf("Table configuration '%s' not found", configName),
			[]string{"Check available table configurations"})
		return
	}

	tableConfig.ApplyDefaults()

	client, err := dynamodb.NewClient(
		tableConfig.Endpoint,
		tableConfig.Region,
		tableConfig.AccessKeyID,
		tableConfig.SecretAccessKey,
	)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}
	defer client.Close()

	if err := client.UpdateItem(tableConfig.TableName, key, req.Updates); err != nil {
		s.sendQueryError(w, err)
		return
	}

	s.sendSuccess(w, map[string]string{"message": "Item updated successfully"})
}

// handleDescribeTable handles GET /api/tables/{config-name}/describe
// Validates: Requirements 1.1, 1.2, 1.3, 1.4, 1.5
func (s *Server) handleDescribeTable(w http.ResponseWriter, r *http.Request, configName string) {
	tableConfig, exists := s.config.GetTableConfig(configName)
	if !exists {
		s.sendError(w, http.StatusNotFound,
			fmt.Sprintf("Table configuration '%s' not found", configName),
			[]string{"Check available table configurations"})
		return
	}

	tableConfig.ApplyDefaults()

	client, err := dynamodb.NewClient(
		tableConfig.Endpoint,
		tableConfig.Region,
		tableConfig.AccessKeyID,
		tableConfig.SecretAccessKey,
	)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}
	defer client.Close()

	meta, err := client.DescribeTable(tableConfig.TableName)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}

	s.sendSuccess(w, meta)
}

// handleQueryTable handles POST /api/tables/{config-name}/query
// Validates: Requirements 7.1, 7.3, 7.4, 7.5, 7.6
func (s *Server) handleQueryTable(w http.ResponseWriter, r *http.Request, configName string) {
	tableConfig, exists := s.config.GetTableConfig(configName)
	if !exists {
		s.sendError(w, http.StatusNotFound,
			fmt.Sprintf("Table configuration '%s' not found", configName),
			[]string{"Check available table configurations"})
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest,
			"Invalid request body",
			[]string{"Ensure request body is valid JSON", "Check query request structure"})
		return
	}

	if req.KeyConditionExpr == "" {
		s.sendError(w, http.StatusBadRequest,
			"Key condition expression is required",
			[]string{"Provide a key_condition_expression with at least a partition key condition"})
		return
	}

	tableConfig.ApplyDefaults()

	client, err := dynamodb.NewClient(
		tableConfig.Endpoint,
		tableConfig.Region,
		tableConfig.AccessKeyID,
		tableConfig.SecretAccessKey,
	)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}
	defer client.Close()

	// Convert expression attribute values from TypedValue to DynamoDB AttributeValue
	var exprAttrValues map[string]*sdkdynamodb.AttributeValue
	if len(req.ExprAttrValues) > 0 {
		exprAttrValues = make(map[string]*sdkdynamodb.AttributeValue, len(req.ExprAttrValues))
		for k, tv := range req.ExprAttrValues {
			av, err := convertTypedToAV(tv)
			if err != nil {
				s.sendError(w, http.StatusBadRequest,
					fmt.Sprintf("Invalid expression attribute value for %s: %s", k, err.Error()),
					[]string{"Check the type and value fields for each expression attribute value"})
				return
			}
			exprAttrValues[k] = av
		}
	}

	// Convert exclusive start key from TypedValue to DynamoDB AttributeValue
	var exclusiveStartKey map[string]*sdkdynamodb.AttributeValue
	if len(req.ExclusiveStartKey) > 0 {
		exclusiveStartKey = make(map[string]*sdkdynamodb.AttributeValue, len(req.ExclusiveStartKey))
		for k, tv := range req.ExclusiveStartKey {
			av, err := convertTypedToAV(tv)
			if err != nil {
				s.sendError(w, http.StatusBadRequest,
					fmt.Sprintf("Invalid exclusive start key value for %s: %s", k, err.Error()),
					[]string{"Check the type and value fields for the exclusive start key"})
				return
			}
			exclusiveStartKey[k] = av
		}
	}

	// Default ScanForward to true if not specified
	scanForward := true
	if req.ScanForward != nil {
		scanForward = *req.ScanForward
	}

	params := dynamodb.QueryParams{
		TableName:         tableConfig.TableName,
		IndexName:         req.IndexName,
		KeyConditionExpr:  req.KeyConditionExpr,
		FilterExpr:        req.FilterExpr,
		ProjectionExpr:    req.ProjectionExpr,
		ExprAttrNames:     req.ExprAttrNames,
		ExprAttrValues:    exprAttrValues,
		ExclusiveStartKey: exclusiveStartKey,
		ScanForward:       scanForward,
		Limit:             req.Limit,
	}

	result, err := client.QueryTable(params)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}

	// Convert result items to typed format
	typedItems := make([]map[string]TypedValue, len(result.Items))
	for i, item := range result.Items {
		typedItems[i] = convertToTypedItem(item)
	}

	// Convert LastEvaluatedKey to typed format
	var lastEvaluatedKey map[string]TypedValue
	if len(result.LastEvaluatedKey) > 0 {
		lastEvaluatedKey = convertToTypedItem(result.LastEvaluatedKey)
	}

	// Build consumed capacity info
	var consumedCapacity *ConsumedCapacityInfo
	if result.ConsumedCapacity != nil {
		consumedCapacity = &ConsumedCapacityInfo{
			TableName:     aws.StringValue(result.ConsumedCapacity.TableName),
			CapacityUnits: aws.Float64Value(result.ConsumedCapacity.CapacityUnits),
		}
	}

	resp := QueryScanResponse{
		Items:            typedItems,
		Count:            result.Count,
		ScannedCount:     result.ScannedCount,
		LastEvaluatedKey: lastEvaluatedKey,
		ConsumedCapacity: consumedCapacity,
	}

	s.sendSuccess(w, resp)
}

// handleScanTable handles POST /api/tables/{config-name}/scan
// Validates: Requirements 7.2, 7.4, 7.5, 7.6
func (s *Server) handleScanTable(w http.ResponseWriter, r *http.Request, configName string) {
	tableConfig, exists := s.config.GetTableConfig(configName)
	if !exists {
		s.sendError(w, http.StatusNotFound,
			fmt.Sprintf("Table configuration '%s' not found", configName),
			[]string{"Check available table configurations"})
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest,
			"Invalid request body",
			[]string{"Ensure request body is valid JSON", "Check scan request structure"})
		return
	}

	tableConfig.ApplyDefaults()

	client, err := dynamodb.NewClient(
		tableConfig.Endpoint,
		tableConfig.Region,
		tableConfig.AccessKeyID,
		tableConfig.SecretAccessKey,
	)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}
	defer client.Close()

	// Convert expression attribute values from TypedValue to DynamoDB AttributeValue
	var exprAttrValues map[string]*sdkdynamodb.AttributeValue
	if len(req.ExprAttrValues) > 0 {
		exprAttrValues = make(map[string]*sdkdynamodb.AttributeValue, len(req.ExprAttrValues))
		for k, tv := range req.ExprAttrValues {
			av, err := convertTypedToAV(tv)
			if err != nil {
				s.sendError(w, http.StatusBadRequest,
					fmt.Sprintf("Invalid expression attribute value for %s: %s", k, err.Error()),
					[]string{"Check the type and value fields for each expression attribute value"})
				return
			}
			exprAttrValues[k] = av
		}
	}

	// Convert exclusive start key from TypedValue to DynamoDB AttributeValue
	var exclusiveStartKey map[string]*sdkdynamodb.AttributeValue
	if len(req.ExclusiveStartKey) > 0 {
		exclusiveStartKey = make(map[string]*sdkdynamodb.AttributeValue, len(req.ExclusiveStartKey))
		for k, tv := range req.ExclusiveStartKey {
			av, err := convertTypedToAV(tv)
			if err != nil {
				s.sendError(w, http.StatusBadRequest,
					fmt.Sprintf("Invalid exclusive start key value for %s: %s", k, err.Error()),
					[]string{"Check the type and value fields for the exclusive start key"})
				return
			}
			exclusiveStartKey[k] = av
		}
	}

	params := dynamodb.ScanParams{
		TableName:         tableConfig.TableName,
		IndexName:         req.IndexName,
		FilterExpr:        req.FilterExpr,
		ProjectionExpr:    req.ProjectionExpr,
		ExprAttrNames:     req.ExprAttrNames,
		ExprAttrValues:    exprAttrValues,
		ExclusiveStartKey: exclusiveStartKey,
		Limit:             req.Limit,
	}

	result, err := client.ScanTableAdvanced(params)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}

	// Convert result items to typed format
	typedItems := make([]map[string]TypedValue, len(result.Items))
	for i, item := range result.Items {
		typedItems[i] = convertToTypedItem(item)
	}

	// Convert LastEvaluatedKey to typed format
	var lastEvaluatedKey map[string]TypedValue
	if len(result.LastEvaluatedKey) > 0 {
		lastEvaluatedKey = convertToTypedItem(result.LastEvaluatedKey)
	}

	// Build consumed capacity info
	var consumedCapacity *ConsumedCapacityInfo
	if result.ConsumedCapacity != nil {
		consumedCapacity = &ConsumedCapacityInfo{
			TableName:     aws.StringValue(result.ConsumedCapacity.TableName),
			CapacityUnits: aws.Float64Value(result.ConsumedCapacity.CapacityUnits),
		}
	}

	resp := QueryScanResponse{
		Items:            typedItems,
		Count:            result.Count,
		ScannedCount:     result.ScannedCount,
		LastEvaluatedKey: lastEvaluatedKey,
		ConsumedCapacity: consumedCapacity,
	}

	s.sendSuccess(w, resp)
}

// handleDeleteItem handles DELETE /api/tables/{config-name}/items/{key}
// Validates: Requirements 8.3, 8.4, 8.5, 10.5
func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request, configName string, encodedKey string) {
	// URL-decode the key
	decodedKey, err := url.PathUnescape(encodedKey)
	if err != nil {
		s.sendError(w, http.StatusBadRequest,
			"Invalid item key encoding",
			[]string{"Ensure the key is properly URL-encoded"})
		return
	}

	// Parse key from JSON
	var key map[string]interface{}
	if err := json.Unmarshal([]byte(decodedKey), &key); err != nil {
		s.sendError(w, http.StatusBadRequest,
			"Invalid item key format",
			[]string{"Key must be a JSON object", "Example: {\"id\": \"42\"}"})
		return
	}

	tableConfig, exists := s.config.GetTableConfig(configName)
	if !exists {
		s.sendError(w, http.StatusNotFound,
			fmt.Sprintf("Table configuration '%s' not found", configName),
			[]string{"Check available table configurations"})
		return
	}

	tableConfig.ApplyDefaults()

	client, err := dynamodb.NewClient(
		tableConfig.Endpoint,
		tableConfig.Region,
		tableConfig.AccessKeyID,
		tableConfig.SecretAccessKey,
	)
	if err != nil {
		s.sendQueryError(w, err)
		return
	}
	defer client.Close()

	if err := client.DeleteItem(tableConfig.TableName, key); err != nil {
		s.sendQueryError(w, err)
		return
	}

	s.sendSuccess(w, map[string]string{"message": "Item deleted successfully"})
}
