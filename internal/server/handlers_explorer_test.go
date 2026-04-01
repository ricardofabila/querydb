package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"querydb/internal/config"
)

var _ = Describe("Handlers Explorer", func() {
	Describe("handleQueryTable", func() {
		It("returns 404 for missing config", func() {
			cfg := &config.Config{Tables: map[string]config.TableConfig{}}
			srv := New(cfg, "127.0.0.1", 8080)

			body, _ := json.Marshal(QueryRequest{KeyConditionExpr: "#pk = :pk"})
			req := httptest.NewRequest(http.MethodPost, "/api/tables/nonexistent/query", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleQueryTable(w, req, "nonexistent")

			Expect(w.Code).To(Equal(http.StatusNotFound))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Table configuration 'nonexistent' not found"))
		})

		It("returns 400 for invalid JSON body", func() {
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

			req := httptest.NewRequest(http.MethodPost, "/api/tables/mydb/query", bytes.NewBufferString("not json"))
			w := httptest.NewRecorder()

			srv.handleQueryTable(w, req, "mydb")

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Invalid request body"))
		})

		It("returns 400 for missing key condition expression", func() {
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

			body, _ := json.Marshal(QueryRequest{KeyConditionExpr: ""})
			req := httptest.NewRequest(http.MethodPost, "/api/tables/mydb/query", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleQueryTable(w, req, "mydb")

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Key condition expression is required"))
		})

		It("returns 405 for GET method via routing", func() {
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

			req := httptest.NewRequest(http.MethodGet, "/api/tables/mydb/query", nil)
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

	Describe("handleScanTable", func() {
		It("returns 404 for missing config", func() {
			cfg := &config.Config{Tables: map[string]config.TableConfig{}}
			srv := New(cfg, "127.0.0.1", 8080)

			body, _ := json.Marshal(ScanRequest{})
			req := httptest.NewRequest(http.MethodPost, "/api/tables/nonexistent/scan", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleScanTable(w, req, "nonexistent")

			Expect(w.Code).To(Equal(http.StatusNotFound))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Table configuration 'nonexistent' not found"))
		})

		It("returns 400 for invalid JSON body", func() {
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

			req := httptest.NewRequest(http.MethodPost, "/api/tables/mydb/scan", bytes.NewBufferString("not json"))
			w := httptest.NewRecorder()

			srv.handleScanTable(w, req, "mydb")

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeFalse())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(Equal("Invalid request body"))
		})

		It("returns 405 for GET method via routing", func() {
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

			req := httptest.NewRequest(http.MethodGet, "/api/tables/mydb/scan", nil)
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

	Describe("Route dispatch for new resources", func() {
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

		DescribeTable("routes resources correctly",
			func(method, path, body string, expectedStatus int, expectMessage string) {
				var req *http.Request
				if body != "" {
					req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
				} else {
					req = httptest.NewRequest(method, path, nil)
				}
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
			Entry("describe with POST returns 405",
				http.MethodPost, "/api/tables/mydb/describe", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("describe with PUT returns 405",
				http.MethodPut, "/api/tables/mydb/describe", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("describe with DELETE returns 405",
				http.MethodDelete, "/api/tables/mydb/describe", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("query with GET returns 405",
				http.MethodGet, "/api/tables/mydb/query", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("query with PUT returns 405",
				http.MethodPut, "/api/tables/mydb/query", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("query with DELETE returns 405",
				http.MethodDelete, "/api/tables/mydb/query", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("scan with GET returns 405",
				http.MethodGet, "/api/tables/mydb/scan", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("scan with PUT returns 405",
				http.MethodPut, "/api/tables/mydb/scan", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("scan with DELETE returns 405",
				http.MethodDelete, "/api/tables/mydb/scan", "", http.StatusMethodNotAllowed, "Method not allowed"),
			Entry("describe for nonexistent config returns 404",
				http.MethodGet, "/api/tables/missing/describe", "",
				http.StatusNotFound, "Table configuration 'missing' not found"),
			Entry("query for nonexistent config returns 404",
				http.MethodPost, "/api/tables/missing/query", `{"key_condition_expression":"#pk = :pk"}`,
				http.StatusNotFound, "Table configuration 'missing' not found"),
			Entry("scan for nonexistent config returns 404",
				http.MethodPost, "/api/tables/missing/scan", `{}`,
				http.StatusNotFound, "Table configuration 'missing' not found"),
		)
	})
})
