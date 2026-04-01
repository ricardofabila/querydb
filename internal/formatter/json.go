package formatter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// QueryResult represents the formatted output structure
type QueryResult struct {
	Summary Summary                  `json:"summary"`
	Records []map[string]interface{} `json:"records"`
}

// Summary contains metadata about the query results
type Summary struct {
	RecordCount int    `json:"record_count"`
	TableName   string `json:"table_name,omitempty"`
}

// FormatJSON formats query results as pretty-printed JSON with summary information
func FormatJSON(data interface{}) (string, error) {
	// Handle different input types
	switch v := data.(type) {
	case []map[string]interface{}:
		return formatRecords(v, "")
	case map[string]interface{}:
		// Single record case
		records := []map[string]interface{}{v}
		return formatRecords(records, "")
	default:
		// Fallback to simple JSON formatting
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal data to JSON: %w", err)
		}
		return string(jsonData), nil
	}
}

// FormatJSONWithSummary formats query results with additional summary information
func FormatJSONWithSummary(records []map[string]interface{}, tableName string) (string, error) {
	return formatRecords(records, tableName)
}

// formatRecords handles the core formatting logic for record arrays
func formatRecords(records []map[string]interface{}, tableName string) (string, error) {
	// Create result structure with summary
	result := QueryResult{
		Summary: Summary{
			RecordCount: len(records),
			TableName:   tableName,
		},
		Records: records,
	}

	// Handle large datasets efficiently by using streaming approach for very large datasets
	if len(records) > 10000 {
		return formatLargeDataset(result)
	}

	// Standard pretty-print formatting
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal query result to JSON: %w", err)
	}

	return string(jsonData), nil
}

// formatLargeDataset handles very large datasets more efficiently
func formatLargeDataset(result QueryResult) (string, error) {
	var builder strings.Builder
	
	// Write opening structure
	builder.WriteString("{\n")
	
	// Write summary
	summaryJSON, err := json.MarshalIndent(result.Summary, "  ", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal summary: %w", err)
	}
	builder.WriteString("  \"summary\": ")
	builder.Write(summaryJSON)
	builder.WriteString(",\n")
	
	// Write records array opening
	builder.WriteString("  \"records\": [\n")
	
	// Write records one by one to avoid loading everything into memory at once
	for i, record := range result.Records {
		recordJSON, err := json.MarshalIndent(record, "    ", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal record %d: %w", i, err)
		}
		
		builder.WriteString("    ")
		builder.Write(recordJSON)
		
		// Add comma if not the last record
		if i < len(result.Records)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	
	// Close records array and main object
	builder.WriteString("  ]\n}")
	
	return builder.String(), nil
}

// FormatSummaryOnly returns just the summary information as JSON
func FormatSummaryOnly(recordCount int, tableName string) (string, error) {
	summary := Summary{
		RecordCount: recordCount,
		TableName:   tableName,
	}
	
	jsonData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal summary to JSON: %w", err)
	}
	
	return string(jsonData), nil
}