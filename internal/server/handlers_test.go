package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"querydb/internal/config"
	qerrors "querydb/internal/errors"
)

var _ = Describe("Handlers", func() {
	Describe("sendSuccess", func() {
		DescribeTable("sends correct success response",
			func(name string, data interface{}) {
				cfg := &config.Config{}
				srv := New(cfg, "127.0.0.1", 8080)

				w := httptest.NewRecorder()
				srv.sendSuccess(w, data)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))

				var resp APIResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.Success).To(BeTrue())
				Expect(resp.Error).To(BeNil())
			},
			Entry("simple string data", "simple string data", "test data"),
			Entry("map data", "map data", map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			}),
			Entry("array data", "array data", []string{"item1", "item2", "item3"}),
			Entry("nil data", "nil data", nil),
		)
	})

	Describe("sendError", func() {
		DescribeTable("sends correct error response",
			func(status int, message string, suggestions []string) {
				cfg := &config.Config{}
				srv := New(cfg, "127.0.0.1", 8080)

				w := httptest.NewRecorder()
				srv.sendError(w, status, message, suggestions)

				Expect(w.Code).To(Equal(status))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))

				var resp APIResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.Success).To(BeFalse())
				Expect(resp.Data).To(BeNil())
				Expect(resp.Error).NotTo(BeNil())
				Expect(resp.Error.Message).To(Equal(message))
				Expect(resp.Error.Suggestions).To(HaveLen(len(suggestions)))
			},
			Entry("bad request with suggestions",
				http.StatusBadRequest, "Invalid request", []string{"Check your input", "Verify the format"}),
			Entry("not found without suggestions",
				http.StatusNotFound, "Resource not found", []string(nil)),
			Entry("internal server error",
				http.StatusInternalServerError, "Something went wrong", []string{"Try again later"}),
		)
	})

	Describe("sendQueryError", func() {
		DescribeTable("maps QueryError to correct HTTP status",
			func(errType qerrors.ErrorType, message string, suggestions []string, expectedStatus int, expectedType string) {
				cfg := &config.Config{}
				srv := New(cfg, "127.0.0.1", 8080)

				w := httptest.NewRecorder()
				qErr := &qerrors.QueryError{
					Type:        errType,
					Message:     message,
					Suggestions: suggestions,
				}
				srv.sendQueryError(w, qErr)

				Expect(w.Code).To(Equal(expectedStatus))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))

				var resp APIResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.Success).To(BeFalse())
				Expect(resp.Error).NotTo(BeNil())
				Expect(resp.Error.Type).To(Equal(expectedType))
				Expect(resp.Error.Message).To(Equal(message))
			},
			Entry("validation error",
				qerrors.ErrorTypeValidation, "Invalid input", []string{"Check the format"},
				http.StatusBadRequest, "validation"),
			Entry("configuration error",
				qerrors.ErrorTypeConfiguration, "Config not found", []string{"Check config file"},
				http.StatusBadRequest, "configuration"),
			Entry("connection error",
				qerrors.ErrorTypeConnection, "Cannot connect to DynamoDB", []string{"Check if service is running"},
				http.StatusServiceUnavailable, "connection"),
			Entry("query error",
				qerrors.ErrorTypeQuery, "Table not found", []string{"Verify table name"},
				http.StatusNotFound, "query"),
			Entry("format error",
				qerrors.ErrorTypeFormat, "JSON parsing failed", []string(nil),
				http.StatusInternalServerError, "format"),
		)

		It("handles non-QueryError with 500 status", func() {
			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			w := httptest.NewRecorder()
			srv.sendQueryError(w, errors.New("generic error"))

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("generic error"))
		})
	})

	Describe("handleGetTables", func() {
		DescribeTable("returns tables based on config and method",
			func(method string, tables map[string]config.TableConfig, expectedStatus int, expectSuccess bool, expectCount int) {
				cfg := &config.Config{Tables: tables}
				srv := New(cfg, "127.0.0.1", 8080)

				req := httptest.NewRequest(method, "/api/tables", nil)
				w := httptest.NewRecorder()

				srv.handleGetTables(w, req)

				Expect(w.Code).To(Equal(expectedStatus))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))

				var resp APIResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.Success).To(Equal(expectSuccess))

				if !expectSuccess {
					return
				}

				dataBytes, err := json.Marshal(resp.Data)
				Expect(err).NotTo(HaveOccurred())

				var tblList []TableInfo
				Expect(json.Unmarshal(dataBytes, &tblList)).To(Succeed())
				Expect(tblList).To(HaveLen(expectCount))

				for _, table := range tblList {
					cfgEntry, exists := tables[table.ConfigName]
					Expect(exists).To(BeTrue())
					Expect(table.TableName).To(Equal(cfgEntry.TableName))
					Expect(table.Endpoint).To(Equal(cfgEntry.Endpoint))
					Expect(table.Region).To(Equal(cfgEntry.Region))
				}
			},
			Entry("returns all configured tables", http.MethodGet,
				map[string]config.TableConfig{
					"users":  {TableName: "users-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
					"orders": {TableName: "orders-table", Endpoint: "http://localhost:8000", Region: "us-west-2"},
				}, http.StatusOK, true, 2),
			Entry("returns empty array when no tables configured", http.MethodGet,
				map[string]config.TableConfig{}, http.StatusOK, true, 0),
			Entry("returns single table", http.MethodGet,
				map[string]config.TableConfig{
					"products": {TableName: "products-table", Endpoint: "http://localhost:4566", Region: "eu-west-1"},
				}, http.StatusOK, true, 1),
			Entry("rejects POST method", http.MethodPost,
				map[string]config.TableConfig{
					"users": {TableName: "users-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				}, http.StatusMethodNotAllowed, false, 0),
			Entry("rejects PUT method", http.MethodPut,
				map[string]config.TableConfig{
					"users": {TableName: "users-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				}, http.StatusMethodNotAllowed, false, 0),
			Entry("rejects DELETE method", http.MethodDelete,
				map[string]config.TableConfig{
					"users": {TableName: "users-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				}, http.StatusMethodNotAllowed, false, 0),
		)

		It("handles nil tables map", func() {
			cfg := &config.Config{Tables: nil}
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodGet, "/api/tables", nil)
			w := httptest.NewRecorder()

			srv.handleGetTables(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeTrue())

			dataBytes, _ := json.Marshal(resp.Data)
			var tables []TableInfo
			Expect(json.Unmarshal(dataBytes, &tables)).To(Succeed())
			Expect(tables).To(HaveLen(0))
		})
	})

	Describe("errorTypeToHTTPStatus", func() {
		DescribeTable("maps error types to HTTP status codes",
			func(errorType qerrors.ErrorType, expectedStatus int) {
				cfg := &config.Config{}
				srv := New(cfg, "127.0.0.1", 8080)
				Expect(srv.errorTypeToHTTPStatus(errorType)).To(Equal(expectedStatus))
			},
			Entry("validation error", qerrors.ErrorTypeValidation, http.StatusBadRequest),
			Entry("configuration error", qerrors.ErrorTypeConfiguration, http.StatusBadRequest),
			Entry("connection error", qerrors.ErrorTypeConnection, http.StatusServiceUnavailable),
			Entry("query error", qerrors.ErrorTypeQuery, http.StatusNotFound),
			Entry("format error", qerrors.ErrorTypeFormat, http.StatusInternalServerError),
			Entry("unknown error type", qerrors.ErrorType("unknown"), http.StatusInternalServerError),
		)
	})

	Describe("handleTableOperations routing", func() {
		var srv *Server

		BeforeEach(func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"mydb": {
						TableName:       "my-table",
						Endpoint:        "http://localhost:8000",
						Region:          "us-east-1",
						AccessKeyID:     "foo",
						SecretAccessKey: "bar",
					},
				},
			}
			srv = New(cfg, "127.0.0.1", 8080)
		})

		DescribeTable("returns correct error for invalid routes",
			func(method, path string, expectedStatus int, expectMessage string) {
				req := httptest.NewRequest(method, path, nil)
				w := httptest.NewRecorder()

				srv.handleTableOperations(w, req)

				Expect(w.Code).To(Equal(expectedStatus))

				var resp APIResponse
				Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
				Expect(resp.Success).To(BeFalse())
				Expect(resp.Error).NotTo(BeNil())
				if expectMessage != "" {
					Expect(resp.Error.Message).To(Equal(expectMessage))
				}
			},
			Entry("invalid path - missing resource",
				http.MethodGet, "/api/tables/mydb",
				http.StatusBadRequest, "Invalid path: expected /api/tables/{config-name}/{resource}"),
			Entry("invalid path - empty config name",
				http.MethodGet, "/api/tables//items",
				http.StatusBadRequest, "Invalid path: expected /api/tables/{config-name}/{resource}"),
			Entry("unknown resource returns 404",
				http.MethodGet, "/api/tables/mydb/unknown",
				http.StatusNotFound, "Resource 'unknown' not found"),
			Entry("unsupported method returns 405",
				http.MethodPatch, "/api/tables/mydb/items",
				http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("PUT without key returns 400",
				http.MethodPut, "/api/tables/mydb/items",
				http.StatusBadRequest, "Item key required for update"),
			Entry("DELETE without key returns 400",
				http.MethodDelete, "/api/tables/mydb/items",
				http.StatusBadRequest, "Item key required for delete"),
		)
	})

	Describe("handleGetItems", func() {
		It("returns 404 for missing config", func() {
			cfg := &config.Config{Tables: map[string]config.TableConfig{}}
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodGet, "/api/tables/nonexistent/items", nil)
			w := httptest.NewRecorder()

			srv.handleGetItems(w, req, "nonexistent")

			Expect(w.Code).To(Equal(http.StatusNotFound))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Table configuration 'nonexistent' not found"))
		})
	})

	Describe("handleCreateItem", func() {
		It("returns 400 for invalid JSON body", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"mydb": {TableName: "my-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				},
			}
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodPost, "/api/tables/mydb/items", bytes.NewBufferString("not json"))
			w := httptest.NewRecorder()

			srv.handleCreateItem(w, req, "mydb")

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Invalid request body"))
		})

		It("returns 400 for empty item data", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"mydb": {TableName: "my-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				},
			}
			srv := New(cfg, "127.0.0.1", 8080)

			body, _ := json.Marshal(CreateItemRequest{Item: map[string]interface{}{}})
			req := httptest.NewRequest(http.MethodPost, "/api/tables/mydb/items", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleCreateItem(w, req, "mydb")

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Item data is empty"))
		})

		It("returns 404 for missing config", func() {
			cfg := &config.Config{Tables: map[string]config.TableConfig{}}
			srv := New(cfg, "127.0.0.1", 8080)

			body, _ := json.Marshal(CreateItemRequest{Item: map[string]interface{}{"id": "1"}})
			req := httptest.NewRequest(http.MethodPost, "/api/tables/nonexistent/items", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleCreateItem(w, req, "nonexistent")

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("handleUpdateItem", func() {
		It("returns 400 for invalid key encoding", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"mydb": {TableName: "my-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				},
			}
			srv := New(cfg, "127.0.0.1", 8080)

			body, _ := json.Marshal(UpdateItemRequest{Updates: map[string]interface{}{"name": "new"}})
			req := httptest.NewRequest(http.MethodPut, "/api/tables/mydb/items/not-json", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleUpdateItem(w, req, "mydb", "not-json")

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Invalid item key format"))
		})

		It("returns 400 for invalid request body", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"mydb": {TableName: "my-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				},
			}
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodPut, "/api/tables/mydb/items/%7B%22id%22%3A%2242%22%7D",
				bytes.NewBufferString("not json"))
			w := httptest.NewRecorder()

			srv.handleUpdateItem(w, req, "mydb", `{"id":"42"}`)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Invalid request body"))
		})

		It("returns 400 for empty updates", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"mydb": {TableName: "my-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				},
			}
			srv := New(cfg, "127.0.0.1", 8080)

			body, _ := json.Marshal(UpdateItemRequest{Updates: map[string]interface{}{}})
			req := httptest.NewRequest(http.MethodPut, "/api/tables/mydb/items/%7B%22id%22%3A%2242%22%7D",
				bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleUpdateItem(w, req, "mydb", `{"id":"42"}`)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Updates data is empty"))
		})

		It("returns 404 for missing config", func() {
			cfg := &config.Config{Tables: map[string]config.TableConfig{}}
			srv := New(cfg, "127.0.0.1", 8080)

			body, _ := json.Marshal(UpdateItemRequest{Updates: map[string]interface{}{"name": "new"}})
			req := httptest.NewRequest(http.MethodPut, "/api/tables/nonexistent/items/%7B%22id%22%3A%2242%22%7D",
				bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleUpdateItem(w, req, "nonexistent", `{"id":"42"}`)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("handleDeleteItem", func() {
		It("returns 400 for invalid key", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"mydb": {TableName: "my-table", Endpoint: "http://localhost:8000", Region: "us-east-1"},
				},
			}
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodDelete, "/api/tables/mydb/items/not-json", nil)
			w := httptest.NewRecorder()

			srv.handleDeleteItem(w, req, "mydb", "not-json")

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Invalid item key format"))
		})

		It("returns 404 for missing config", func() {
			cfg := &config.Config{Tables: map[string]config.TableConfig{}}
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodDelete, "/api/tables/nonexistent/items/%7B%22id%22%3A%2242%22%7D", nil)
			w := httptest.NewRecorder()

			srv.handleDeleteItem(w, req, "nonexistent", `{"id":"42"}`)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("handleDescribeTable", func() {
		It("returns 404 for missing config", func() {
			cfg := &config.Config{Tables: map[string]config.TableConfig{}}
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodGet, "/api/tables/nonexistent/describe", nil)
			w := httptest.NewRecorder()

			srv.handleDescribeTable(w, req, "nonexistent")

			Expect(w.Code).To(Equal(http.StatusNotFound))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Table configuration 'nonexistent' not found"))
		})

		It("returns 405 for POST method via routing", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"mydb": {
						TableName:       "my-table",
						Endpoint:        "http://localhost:8000",
						Region:          "us-east-1",
						AccessKeyID:     "foo",
						SecretAccessKey: "bar",
					},
				},
			}
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodPost, "/api/tables/mydb/describe", nil)
			w := httptest.NewRecorder()

			srv.handleTableOperations(w, req)

			Expect(w.Code).To(Equal(http.StatusMethodNotAllowed))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Method not allowed"))
		})
	})
})
