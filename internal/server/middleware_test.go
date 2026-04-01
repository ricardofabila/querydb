package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"querydb/internal/config"
)

var _ = Describe("Middleware", func() {
	Describe("CORS Middleware", func() {
		Context("with localhost binding", func() {
			DescribeTable("handles origins correctly",
				func(origin string, expectAllow bool, expectedOrigin string) {
					cfg := &config.Config{}
					srv := New(cfg, "127.0.0.1", 8080)

					handler := srv.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					}))

					req := httptest.NewRequest("GET", "/api/test", nil)
					if origin != "" {
						req.Header.Set("Origin", origin)
					}
					rec := httptest.NewRecorder()

					handler.ServeHTTP(rec, req)

					allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
					if expectAllow {
						Expect(allowOrigin).To(Equal(expectedOrigin))
					} else {
						Expect(allowOrigin).To(BeEmpty())
					}

					Expect(rec.Header().Get("Access-Control-Allow-Methods")).To(Equal("GET, POST, PUT, DELETE, OPTIONS"))
					Expect(rec.Header().Get("Access-Control-Allow-Headers")).To(Equal("Content-Type"))
				},
				Entry("localhost HTTP origin allowed", "http://localhost:3000", true, "http://localhost:3000"),
				Entry("localhost HTTPS origin allowed", "https://localhost:3000", true, "https://localhost:3000"),
				Entry("127.0.0.1 HTTP origin allowed", "http://127.0.0.1:3000", true, "http://127.0.0.1:3000"),
				Entry("127.0.0.1 HTTPS origin allowed", "https://127.0.0.1:3000", true, "https://127.0.0.1:3000"),
				Entry("external origin not allowed", "http://example.com", false, ""),
				Entry("no origin header", "", false, ""),
			)
		})

		Context("with all-interfaces binding", func() {
			DescribeTable("allows all origins",
				func(origin string, expectedOrigin string) {
					cfg := &config.Config{}
					srv := New(cfg, "0.0.0.0", 8080)

					handler := srv.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					}))

					req := httptest.NewRequest("GET", "/api/test", nil)
					if origin != "" {
						req.Header.Set("Origin", origin)
					}
					rec := httptest.NewRecorder()

					handler.ServeHTTP(rec, req)

					Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal(expectedOrigin))
				},
				Entry("localhost origin allowed", "http://localhost:3000", "http://localhost:3000"),
				Entry("external origin allowed", "http://example.com", "http://example.com"),
				Entry("any origin allowed", "https://app.example.com:8443", "https://app.example.com:8443"),
				Entry("no origin header defaults to wildcard", "", "*"),
			)
		})

		It("handles preflight OPTIONS request", func() {
			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			nextCalled := false
			handler := srv.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("OPTIONS", "/api/test", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(nextCalled).To(BeFalse())
			Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal("http://localhost:3000"))
		})

		It("calls next handler for non-OPTIONS request", func() {
			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			nextCalled := false
			handler := srv.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(nextCalled).To(BeTrue())
			Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal("http://localhost:3000"))
		})
	})

	Describe("Logging Middleware", func() {
		It("logs request details", func() {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			handler := srv.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(10 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/api/tables", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			Expect(output).To(ContainSubstring("GET"))
			Expect(output).To(ContainSubstring("/api/tables"))
			Expect(output).To(ContainSubstring(time.Now().Format("2006-01-02")))
			Expect(output).To(ContainSubstring("Completed in"))
		})

		It("logs duration", func() {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			sleepDuration := 10 * time.Millisecond
			handler := srv.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(sleepDuration)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("POST", "/api/tables/test/items", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			Expect(output).To(ContainSubstring("Completed in"))
			Expect(output).To(SatisfyAny(ContainSubstring("ms"), ContainSubstring("s")))
		})

		It("logs multiple requests", func() {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			handler := srv.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			methods := []string{"GET", "POST", "PUT", "DELETE"}
			paths := []string{"/api/tables", "/api/tables/test/items", "/api/tables/test/items/123", "/"}

			for i, method := range methods {
				req := httptest.NewRequest(method, paths[i], nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
			}

			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			for i, method := range methods {
				Expect(output).To(ContainSubstring(method))
				Expect(output).To(ContainSubstring(paths[i]))
			}

			Expect(strings.Count(output, "Completed in")).To(Equal(4))
		})

		It("calls next handler", func() {
			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			nextCalled := false
			handler := srv.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			oldStdout := os.Stdout
			os.Stdout, _ = os.Open(os.DevNull)
			defer func() { os.Stdout = oldStdout }()

			req := httptest.NewRequest("GET", "/api/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(nextCalled).To(BeTrue())
		})

		It("uses correct timestamp format", func() {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			handler := srv.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			now := time.Now()
			expectedDate := now.Format("2006-01-02")
			expectedTimePattern := fmt.Sprintf("[%s", expectedDate)

			Expect(output).To(ContainSubstring(expectedTimePattern))

			lines := strings.Split(output, "\n")
			if len(lines) > 0 {
				firstLine := lines[0]
				Expect(firstLine).To(HavePrefix("["))
				Expect(firstLine).To(ContainSubstring("]"))
			}
		})
	})

	Describe("Recovery Middleware", func() {
		It("catches panic and returns 500", func() {
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			handler := srv.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			}))

			req := httptest.NewRequest("GET", "/api/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			w.Close()
			os.Stderr = oldStderr
			var buf bytes.Buffer
			io.Copy(&buf, r)
			stderrOutput := buf.String()

			Expect(stderrOutput).To(ContainSubstring("[PANIC]"))
			Expect(stderrOutput).To(ContainSubstring("test panic"))

			Expect(rec.Code).To(Equal(http.StatusInternalServerError))

			var apiResp APIResponse
			Expect(json.NewDecoder(rec.Body).Decode(&apiResp)).To(Succeed())
			Expect(apiResp.Success).To(BeFalse())
			Expect(apiResp.Error).NotTo(BeNil())
			Expect(apiResp.Error.Message).To(Equal("Internal server error"))
			Expect(apiResp.Error.Suggestions).NotTo(BeEmpty())
			Expect(apiResp.Error.Suggestions[0]).To(ContainSubstring("Check server logs"))
		})

		It("does not interfere with normal requests", func() {
			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			nextCalled := false
			handler := srv.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}))

			req := httptest.NewRequest("GET", "/api/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(nextCalled).To(BeTrue())
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("success"))
		})

		It("handles string panic", func() {
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			handler := srv.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("string panic message")
			}))

			req := httptest.NewRequest("POST", "/api/tables/test/items", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			w.Close()
			os.Stderr = oldStderr
			var buf bytes.Buffer
			io.Copy(&buf, r)
			stderrOutput := buf.String()

			Expect(stderrOutput).To(ContainSubstring("string panic message"))
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})

		It("handles error panic", func() {
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			handler := srv.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(fmt.Errorf("error panic: %s", "something went wrong"))
			}))

			req := httptest.NewRequest("DELETE", "/api/tables/test/items/123", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			w.Close()
			os.Stderr = oldStderr
			var buf bytes.Buffer
			io.Copy(&buf, r)
			stderrOutput := buf.String()

			Expect(stderrOutput).To(ContainSubstring("[PANIC]"))
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
		})

		It("returns correct response structure on panic", func() {
			oldStderr := os.Stderr
			os.Stderr, _ = os.Open(os.DevNull)
			defer func() { os.Stderr = oldStderr }()

			cfg := &config.Config{}
			srv := New(cfg, "127.0.0.1", 8080)

			handler := srv.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("test")
			}))

			req := httptest.NewRequest("GET", "/", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			var apiResp APIResponse
			Expect(json.NewDecoder(rec.Body).Decode(&apiResp)).To(Succeed())

			Expect(apiResp.Success).To(BeFalse())
			Expect(apiResp.Data).To(BeNil())
			Expect(apiResp.Error).NotTo(BeNil())
			Expect(apiResp.Error.Type).To(Equal("error"))
			Expect(apiResp.Error.Message).NotTo(BeEmpty())
			Expect(apiResp.Error.Suggestions).NotTo(BeNil())
		})
	})
})
