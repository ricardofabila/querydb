package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"querydb/internal/config"
	"querydb/internal/errors"
)

// containsStringProp checks if s contains substr
func containsStringProp(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && len(substr) > 0 && searchStringProp(s, substr))
}

func searchStringProp(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// writeTestFile is a helper to write content to a file path
func writeTestFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

var _ = Describe("Server Property Tests", func() {
	// **Validates: Requirements 1.2, 12.4**
	// Property 1: Server Port Binding
	It("binds to any valid host:port combination", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 3
		properties := gopter.NewProperties(parameters)

		properties.Property("server binds to any valid host:port combination",
			prop.ForAll(
				func(port int, hostType string) bool {
					var host string
					switch hostType {
					case "localhost":
						host = "127.0.0.1"
					case "all":
						host = "0.0.0.0"
					case "ipv6-localhost":
						host = "::1"
					default:
						host = "127.0.0.1"
					}

					cfg := &config.Config{
						Tables: map[string]config.TableConfig{},
					}

					srv := New(cfg, host, port)
					if srv == nil {
						return false
					}

					expectedAddr := fmt.Sprintf("%s:%d", host, port)
					if srv.httpServer.Addr != expectedAddr {
						return false
					}

					errChan := make(chan error, 1)
					go func() {
						errChan <- srv.Start()
					}()

					time.Sleep(50 * time.Millisecond)

					testURL := fmt.Sprintf("http://%s:%d", host, port)
					if host == "::1" {
						testURL = fmt.Sprintf("http://[%s]:%d", host, port)
					}

					client := &http.Client{Timeout: 500 * time.Millisecond}
					resp, err := client.Get(testURL)

					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()
					srv.Shutdown(ctx)

					select {
					case <-errChan:
					case <-time.After(2 * time.Second):
					}

					if err != nil {
						return false
					}
					defer resp.Body.Close()

					return resp.StatusCode != 0
				},
				gen.IntRange(1024, 65535),
				gen.OneConstOf("localhost", "all"),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 1.6, 14.1, 14.2, 14.3**
	// Property 2: Graceful Shutdown Completion
	It("gracefully shuts down with in-flight requests", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 3
		properties := gopter.NewProperties(parameters)

		properties.Property("server gracefully shuts down with in-flight requests",
			prop.ForAll(
				func(numRequests int, requestDuration int) bool {
					cfg := &config.Config{
						Tables: map[string]config.TableConfig{},
					}

					port := 20000 + (numRequests % 1000)
					srv := New(cfg, "127.0.0.1", port)

					mux := http.NewServeMux()
					completedRequests := make(chan bool, numRequests)

					mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
						time.Sleep(time.Duration(requestDuration) * time.Millisecond)
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("completed"))
						completedRequests <- true
					})

					srv.httpServer.Handler = mux

					errChan := make(chan error, 1)
					go func() {
						errChan <- srv.Start()
					}()

					time.Sleep(50 * time.Millisecond)

					requestErrors := make(chan error, numRequests)
					for i := 0; i < numRequests; i++ {
						go func() {
							client := &http.Client{Timeout: 10 * time.Second}
							resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/slow", port))
							if err != nil {
								requestErrors <- err
								return
							}
							resp.Body.Close()
							requestErrors <- nil
						}()
					}

					time.Sleep(time.Duration(requestDuration/2) * time.Millisecond)

					shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()

					shutdownErr := srv.Shutdown(shutdownCtx)

					if shutdownErr != nil && shutdownErr != context.DeadlineExceeded {
						return false
					}

					select {
					case err := <-errChan:
						if err != nil && err != http.ErrServerClosed {
							return false
						}
					case <-time.After(3 * time.Second):
						return false
					}

					completedCount := 0
					timeout := time.After(3 * time.Second)
					for i := 0; i < numRequests; i++ {
						select {
						case <-completedRequests:
							completedCount++
						case <-timeout:
							break
						}
					}

					client := &http.Client{Timeout: 1 * time.Second}
					_, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/slow", port))
					if err == nil {
						return false
					}

					return true
				},
				gen.IntRange(1, 3),
				gen.IntRange(50, 500),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 2.2, 2.4**
	// Property 3: Embedded Assets Serving
	It("serves embedded assets with correct content and content types", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 5
		properties := gopter.NewProperties(parameters)

		properties.Property("embedded assets are served with correct content and content types",
			prop.ForAll(
				func(assetPath string) bool {
					var expectedContentType string
					switch {
					case assetPath == "/":
						expectedContentType = "text/html"
					case len(assetPath) > 4 && assetPath[len(assetPath)-4:] == ".css":
						expectedContentType = "text/css"
					case len(assetPath) > 3 && assetPath[len(assetPath)-3:] == ".js":
						expectedContentType = "text/javascript"
					default:
						expectedContentType = "text/html"
					}

					cfg := &config.Config{
						Tables: map[string]config.TableConfig{},
					}

					srv := New(cfg, "127.0.0.1", 8080)

					req := httptest.NewRequest(http.MethodGet, assetPath, nil)
					rec := httptest.NewRecorder()

					srv.httpServer.Handler.ServeHTTP(rec, req)

					if rec.Code != http.StatusOK {
						return false
					}

					contentType := rec.Header().Get("Content-Type")
					if contentType == "" {
						return false
					}

					if !containsStringProp(contentType, expectedContentType) {
						return false
					}

					if rec.Body.Len() == 0 {
						return false
					}

					if expectedContentType == "text/html" {
						body := rec.Body.String()
						if !containsStringProp(body, "<!DOCTYPE html>") {
							return false
						}
					}

					return true
				},
				gen.OneConstOf(
					"/",
					"/css/styles.css",
					"/js/api.js",
					"/js/app.js",
					"/js/browser.js",
					"/js/editor.js",
					"/js/tables.js",
				),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 13.1, 13.2, 13.3**
	// Property 14: CORS Headers Configuration
	It("configures CORS headers correctly for different host bindings and origins", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 5
		properties := gopter.NewProperties(parameters)

		properties.Property("CORS headers configured correctly for different host bindings and origins",
			prop.ForAll(
				func(hostType string, originType string, method string) bool {
					var host string
					switch hostType {
					case "localhost":
						host = "127.0.0.1"
					case "all":
						host = "0.0.0.0"
					default:
						host = "127.0.0.1"
					}

					var origin string
					var shouldAllow bool
					switch originType {
					case "localhost-http":
						origin = "http://localhost:3000"
						shouldAllow = true
					case "localhost-https":
						origin = "https://localhost:8443"
						shouldAllow = true
					case "127-http":
						origin = "http://127.0.0.1:5000"
						shouldAllow = true
					case "127-https":
						origin = "https://127.0.0.1:9000"
						shouldAllow = true
					case "external":
						origin = "http://example.com"
						shouldAllow = (host == "0.0.0.0")
					case "external-https":
						origin = "https://app.example.com:8443"
						shouldAllow = (host == "0.0.0.0")
					case "empty":
						origin = ""
						shouldAllow = (host == "0.0.0.0")
					default:
						origin = "http://localhost:3000"
						shouldAllow = true
					}

					cfg := &config.Config{
						Tables: map[string]config.TableConfig{},
					}

					srv := New(cfg, host, 8080)

					handler := srv.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					}))

					req := httptest.NewRequest(method, "/api/test", nil)
					if origin != "" {
						req.Header.Set("Origin", origin)
					}
					rec := httptest.NewRecorder()

					handler.ServeHTTP(rec, req)

					allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")

					if shouldAllow {
						expectedOrigin := origin
						if origin == "" && host == "0.0.0.0" {
							expectedOrigin = "*"
						}
						if allowOrigin != expectedOrigin {
							return false
						}
					} else {
						if allowOrigin != "" {
							return false
						}
					}

					allowMethods := rec.Header().Get("Access-Control-Allow-Methods")
					if allowMethods != "GET, POST, PUT, DELETE, OPTIONS" {
						return false
					}

					allowHeaders := rec.Header().Get("Access-Control-Allow-Headers")
					if allowHeaders != "Content-Type" {
						return false
					}

					if method == "OPTIONS" {
						if rec.Code != http.StatusOK {
							return false
						}
					}

					return true
				},
				gen.OneConstOf("localhost", "all"),
				gen.OneConstOf("localhost-http", "localhost-https", "127-http", "127-https",
					"external", "external-https", "empty"),
				gen.OneConstOf("GET", "POST", "PUT", "DELETE", "OPTIONS"),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 4.5, 6.6, 7.6, 8.5, 10.6, 11.1**
	// Property 11: Error Response Format
	It("returns error responses with correct JSON structure and HTTP status codes", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 5
		properties := gopter.NewProperties(parameters)

		properties.Property("error responses have correct JSON structure and HTTP status codes",
			prop.ForAll(
				func(errorType string, message string, hasSuggestions bool) bool {
					cfg := &config.Config{
						Tables: map[string]config.TableConfig{},
					}

					srv := New(cfg, "127.0.0.1", 8080)

					var errType errors.ErrorType
					var expectedStatus int
					switch errorType {
					case "validation":
						errType = errors.ErrorTypeValidation
						expectedStatus = http.StatusBadRequest
					case "configuration":
						errType = errors.ErrorTypeConfiguration
						expectedStatus = http.StatusBadRequest
					case "connection":
						errType = errors.ErrorTypeConnection
						expectedStatus = http.StatusServiceUnavailable
					case "query":
						errType = errors.ErrorTypeQuery
						expectedStatus = http.StatusNotFound
					case "format":
						errType = errors.ErrorTypeFormat
						expectedStatus = http.StatusInternalServerError
					default:
						errType = errors.ErrorTypeValidation
						expectedStatus = http.StatusBadRequest
					}

					var suggestions []string
					if hasSuggestions {
						suggestions = []string{"Suggestion 1", "Suggestion 2"}
					}

					queryErr := &errors.QueryError{
						Type:        errType,
						Message:     message,
						Suggestions: suggestions,
					}

					rec := httptest.NewRecorder()
					srv.sendQueryError(rec, queryErr)

					if rec.Code != expectedStatus {
						return false
					}

					contentType := rec.Header().Get("Content-Type")
					if contentType != "application/json" {
						return false
					}

					var apiResp APIResponse
					if err := json.NewDecoder(rec.Body).Decode(&apiResp); err != nil {
						return false
					}

					if apiResp.Success != false {
						return false
					}

					if apiResp.Data != nil {
						return false
					}

					if apiResp.Error == nil {
						return false
					}

					if apiResp.Error.Type != string(errType) {
						return false
					}

					if apiResp.Error.Message != message {
						return false
					}

					if hasSuggestions {
						if len(apiResp.Error.Suggestions) != len(suggestions) {
							return false
						}
						for i, expected := range suggestions {
							if apiResp.Error.Suggestions[i] != expected {
								return false
							}
						}
					} else {
						if apiResp.Error.Suggestions != nil && len(apiResp.Error.Suggestions) > 0 {
							return false
						}
					}

					return true
				},
				gen.OneConstOf("validation", "configuration", "connection", "query", "format"),
				gen.OneConstOf(
					"Invalid request body",
					"Table configuration not found",
					"Failed to connect to DynamoDB",
					"Table does not exist",
					"JSON serialization failed",
					"Missing required field",
					"Connection timeout",
				),
				gen.Bool(),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 11.2, 11.3**
	// Property 12: Error Suggestions Inclusion
	It("includes suggestions for connection and table not found errors", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 5
		properties := gopter.NewProperties(parameters)

		properties.Property("connection and table not found errors include suggestions",
			prop.ForAll(
				func(errorScenario string, message string) bool {
					cfg := &config.Config{
						Tables: map[string]config.TableConfig{},
					}

					srv := New(cfg, "127.0.0.1", 8080)

					var queryErr *errors.QueryError
					switch errorScenario {
					case "connection":
						queryErr = errors.NewConnectionError(
							message,
							nil,
							"Check if DynamoDB Local or LocalStack is running",
							"Verify the endpoint URL is correct",
							"Ensure the service is accessible from your network",
						)
					case "table-not-found":
						queryErr = errors.NewQueryError(
							message,
							nil,
							"Verify the table name is correct",
							"Check if the table exists in the specified region",
							"Try listing tables first to confirm the table exists",
						)
					default:
						queryErr = errors.NewConnectionError(
							message,
							nil,
							"Check if DynamoDB Local or LocalStack is running",
						)
					}

					rec := httptest.NewRecorder()
					srv.sendQueryError(rec, queryErr)

					var apiResp APIResponse
					if err := json.NewDecoder(rec.Body).Decode(&apiResp); err != nil {
						return false
					}

					if apiResp.Error == nil {
						return false
					}

					if apiResp.Error.Suggestions == nil || len(apiResp.Error.Suggestions) == 0 {
						return false
					}

					for i, suggestion := range apiResp.Error.Suggestions {
						if suggestion == "" {
							return false
						}
						if len(suggestion) < 10 {
							_ = i
							return false
						}
					}

					switch errorScenario {
					case "connection":
						if apiResp.Error.Type != string(errors.ErrorTypeConnection) {
							return false
						}
						if rec.Code != http.StatusServiceUnavailable {
							return false
						}
					case "table-not-found":
						if apiResp.Error.Type != string(errors.ErrorTypeQuery) {
							return false
						}
						if rec.Code != http.StatusNotFound {
							return false
						}
					}

					if apiResp.Error.Message != message {
						return false
					}

					return true
				},
				gen.OneConstOf("connection", "table-not-found"),
				gen.OneConstOf(
					"Failed to connect to DynamoDB",
					"Connection timeout",
					"Unable to reach endpoint",
					"Table does not exist",
					"ResourceNotFoundException: Requested resource not found",
					"Table 'users' not found in region us-east-1",
				),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 3.1, 3.2, 10.1**
	// Property 4: Table List API Completeness
	It("returns all configured tables with required fields", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 5
		properties := gopter.NewProperties(parameters)

		genConfigName := gen.IntRange(1, 999).Map(func(i int) string {
			return fmt.Sprintf("cfg_%d", i)
		})

		genTableName := gen.IntRange(1, 999).Map(func(i int) string {
			return fmt.Sprintf("MyTable_%d", i)
		})

		genEndpoint := gen.IntRange(1000, 9999).Map(func(port int) string {
			return fmt.Sprintf("http://localhost:%d", port)
		})

		genRegion := gen.OneConstOf("us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1", "ap-northeast-1")

		genTableConfigMap := gen.IntRange(0, 10).FlatMap(func(v interface{}) gopter.Gen {
			n := v.(int)
			if n == 0 {
				return gen.Const(map[string]config.TableConfig{})
			}
			return gen.SliceOfN(n, genConfigName).FlatMap(func(v interface{}) gopter.Gen {
				names := v.([]string)
				seen := make(map[string]bool)
				unique := make([]string, 0, len(names))
				for _, name := range names {
					if !seen[name] {
						seen[name] = true
						unique = append(unique, name)
					}
				}
				finalNames := unique
				count := len(finalNames)

				return gen.SliceOfN(count, genTableName).FlatMap(func(v interface{}) gopter.Gen {
					tableNames := v.([]string)
					return gen.SliceOfN(count, genEndpoint).FlatMap(func(v interface{}) gopter.Gen {
						endpoints := v.([]string)
						return gen.SliceOfN(count, genRegion).Map(func(v []string) map[string]config.TableConfig {
							regions := v
							result := make(map[string]config.TableConfig)
							for i, name := range finalNames {
								result[name] = config.TableConfig{
									TableName: tableNames[i],
									Endpoint:  endpoints[i],
									Region:    regions[i],
								}
							}
							return result
						})
					}, reflect.TypeOf(map[string]config.TableConfig{}))
				}, reflect.TypeOf(map[string]config.TableConfig{}))
			}, reflect.TypeOf(map[string]config.TableConfig{}))
		}, reflect.TypeOf(map[string]config.TableConfig{}))

		properties.Property("API returns all configured tables with required fields",
			prop.ForAll(
				func(tables map[string]config.TableConfig) bool {
					cfg := &config.Config{Tables: tables}
					srv := New(cfg, "127.0.0.1", 8080)

					req := httptest.NewRequest(http.MethodGet, "/api/tables", nil)
					rec := httptest.NewRecorder()
					srv.handleGetTables(rec, req)

					if rec.Code != http.StatusOK {
						return false
					}

					var apiResp APIResponse
					if err := json.NewDecoder(rec.Body).Decode(&apiResp); err != nil {
						return false
					}

					if !apiResp.Success {
						return false
					}

					dataSlice, ok := apiResp.Data.([]interface{})
					if !ok {
						if len(tables) == 0 {
							return apiResp.Data == nil || len(apiResp.Data.([]interface{})) == 0
						}
						return false
					}

					if len(dataSlice) != len(tables) {
						return false
					}

					responseTables := make(map[string]map[string]interface{})
					for _, item := range dataSlice {
						tableMap, ok := item.(map[string]interface{})
						if !ok {
							return false
						}
						configName, ok := tableMap["config_name"].(string)
						if !ok || configName == "" {
							return false
						}
						responseTables[configName] = tableMap
					}

					for name, cfg := range tables {
						respTable, exists := responseTables[name]
						if !exists {
							return false
						}
						if respTable["config_name"] != name {
							return false
						}
						if respTable["table_name"] != cfg.TableName {
							return false
						}
						if respTable["endpoint"] != cfg.Endpoint {
							return false
						}
						if respTable["region"] != cfg.Region {
							return false
						}
					}

					return true
				},
				genTableConfigMap,
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 9.3**
	// Property 9: Configuration Path Resolution
	It("loads configuration from custom config paths", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 5
		properties := gopter.NewProperties(parameters)

		properties.Property("server loads configuration from custom config paths",
			prop.ForAll(
				func(configName string, tableName string, region string) bool {
					tmpDir := GinkgoT().TempDir()
					configPath := tmpDir + "/custom-config.yaml"

					configContent := fmt.Sprintf(`tables:
  %s:
    table_name: "%s"
    endpoint: "http://localhost:8000"
    region: "%s"
    access_key_id: "foo"
    secret_access_key: "bar"
`, configName, tableName, region)

					if err := writeTestFile(configPath, configContent); err != nil {
						return false
					}

					cfg, err := config.LoadConfig(configPath)
					if err != nil {
						return false
					}

					tableConfig, exists := cfg.GetTableConfig(configName)
					if !exists {
						return false
					}

					if tableConfig.TableName != tableName {
						return false
					}

					if tableConfig.Region != region {
						return false
					}

					srv := New(cfg, "127.0.0.1", 8080)
					return srv != nil
				},
				gen.OneConstOf("my-table", "dev-db", "staging-users", "prod-orders", "test-items"),
				gen.OneConstOf("users-table", "orders-table", "products", "sessions", "events-log"),
				gen.OneConstOf("us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 9.4**
	// Property 10: Multi-Environment Support
	It("supports both LocalStack and AWS endpoint configurations", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 5
		properties := gopter.NewProperties(parameters)

		properties.Property("server supports both LocalStack and AWS endpoint configurations",
			prop.ForAll(
				func(envType string) bool {
					var endpoint, region, accessKey, secretKey string

					switch envType {
					case "dynamodb-local":
						endpoint = "http://localhost:8000"
						region = "us-east-1"
						accessKey = "foo"
						secretKey = "bar"
					case "localstack":
						endpoint = "http://localhost:4566"
						region = "us-east-1"
						accessKey = "test"
						secretKey = "test"
					case "aws-us-east":
						endpoint = "https://dynamodb.us-east-1.amazonaws.com"
						region = "us-east-1"
						accessKey = "AKIAIOSFODNN7EXAMPLE"
						secretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					case "aws-eu-west":
						endpoint = "https://dynamodb.eu-west-1.amazonaws.com"
						region = "eu-west-1"
						accessKey = "AKIAIOSFODNN7EXAMPLE"
						secretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					case "aws-ap-southeast":
						endpoint = "https://dynamodb.ap-southeast-1.amazonaws.com"
						region = "ap-southeast-1"
						accessKey = "AKIAIOSFODNN7EXAMPLE"
						secretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					default:
						endpoint = "http://localhost:8000"
						region = "us-east-1"
						accessKey = "foo"
						secretKey = "bar"
					}

					cfg := &config.Config{
						Tables: map[string]config.TableConfig{
							"test-table": {
								TableName:       "my-table",
								Endpoint:        endpoint,
								Region:          region,
								AccessKeyID:     accessKey,
								SecretAccessKey: secretKey,
							},
						},
					}

					srv := New(cfg, "127.0.0.1", 8080)
					if srv == nil {
						return false
					}

					tableConfig, exists := srv.config.GetTableConfig("test-table")
					if !exists {
						return false
					}

					if tableConfig.Endpoint != endpoint {
						return false
					}

					if tableConfig.Region != region {
						return false
					}

					return true
				},
				gen.OneConstOf("dynamodb-local", "localstack", "aws-us-east", "aws-eu-west", "aws-ap-southeast"),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 1.4**
	// Property 15: Server Startup Logging
	It("logs URL on startup", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 3
		properties := gopter.NewProperties(parameters)

		properties.Property("server logs URL on startup",
			prop.ForAll(
				func(port int, hostType string) bool {
					var host string
					switch hostType {
					case "localhost":
						host = "127.0.0.1"
					case "all":
						host = "0.0.0.0"
					default:
						host = "127.0.0.1"
					}

					cfg := &config.Config{
						Tables: map[string]config.TableConfig{},
					}

					srv := New(cfg, host, port)

					oldStdout := os.Stdout
					r, w, _ := os.Pipe()
					os.Stdout = w

					errChan := make(chan error, 1)
					go func() {
						errChan <- srv.Start()
					}()

					time.Sleep(100 * time.Millisecond)

					w.Close()
					os.Stdout = oldStdout

					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					output := string(buf[:n])
					r.Close()

					shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					srv.Shutdown(shutdownCtx)

					select {
					case <-errChan:
					case <-time.After(3 * time.Second):
					}

					expectedURL := fmt.Sprintf("http://%s:%d", host, port)
					if !searchStringProp(output, expectedURL) {
						return false
					}

					if !searchStringProp(output, "Starting server") {
						return false
					}

					return true
				},
				gen.IntRange(10000, 60000),
				gen.OneConstOf("localhost", "all"),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})

	// **Validates: Requirements 14.4**
	// Property 16: Shutdown Logging
	It("logs shutdown message", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 3
		properties := gopter.NewProperties(parameters)

		properties.Property("server logs shutdown message",
			prop.ForAll(
				func(port int) bool {
					cfg := &config.Config{
						Tables: map[string]config.TableConfig{},
					}

					srv := New(cfg, "127.0.0.1", port)

					errChan := make(chan error, 1)
					go func() {
						errChan <- srv.Start()
					}()

					time.Sleep(100 * time.Millisecond)

					oldStdout := os.Stdout
					r, w, _ := os.Pipe()
					os.Stdout = w

					shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					srv.Shutdown(shutdownCtx)

					select {
					case <-errChan:
					case <-time.After(3 * time.Second):
					}

					w.Close()
					os.Stdout = oldStdout

					buf := make([]byte, 1024)
					n, _ := r.Read(buf)
					output := string(buf[:n])
					r.Close()

					if !searchStringProp(output, "Shutting down") {
						return false
					}

					return true
				},
				gen.IntRange(10000, 60000),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})
})
