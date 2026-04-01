package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"querydb/internal/config"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

var _ = Describe("Server", func() {
	Describe("New", func() {
		var cfg *config.Config

		BeforeEach(func() {
			cfg = &config.Config{
				Tables: map[string]config.TableConfig{
					"test-table": {
						TableName: "TestTable",
						Endpoint:  "http://localhost:8000",
						Region:    "us-east-1",
					},
				},
			}
		})

		DescribeTable("creates server with correct host and port",
			func(host string, port int) {
				srv := New(cfg, host, port)

				Expect(srv).NotTo(BeNil())
				Expect(srv.config).To(Equal(cfg))
				Expect(srv.host).To(Equal(host))
				Expect(srv.port).To(Equal(port))
				Expect(srv.httpServer).NotTo(BeNil())
				Expect(srv.httpServer.Addr).NotTo(BeEmpty())
			},
			Entry("localhost with standard port", "127.0.0.1", 8080),
			Entry("all interfaces with custom port", "0.0.0.0", 3000),
			Entry("localhost with high port", "127.0.0.1", 9999),
		)
	})

	Describe("ServerLifecycle", func() {
		It("starts and shuts down gracefully", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{},
			}

			srv := New(cfg, "127.0.0.1", 18080)

			errChan := make(chan error, 1)
			go func() {
				errChan <- srv.Start()
			}()

			time.Sleep(100 * time.Millisecond)

			client := &http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get("http://127.0.0.1:18080")
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			Expect(srv.Shutdown(ctx)).To(Succeed())

			select {
			case err := <-errChan:
				Expect(err).To(Equal(http.ErrServerClosed))
			case <-time.After(1 * time.Second):
				Fail("Server did not stop within timeout")
			}
		})
	})

	Describe("ShutdownTimeout", func() {
		It("handles very short timeout without panicking", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{},
			}

			srv := New(cfg, "127.0.0.1", 18081)

			go srv.Start()
			time.Sleep(100 * time.Millisecond)

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			// Shutdown should complete even with short timeout, should not panic
			_ = srv.Shutdown(ctx)
		})
	})

	Describe("ServerFields", func() {
		It("initializes all fields correctly", func() {
			cfg := &config.Config{
				Tables: map[string]config.TableConfig{
					"table1": {TableName: "Table1"},
				},
			}

			srv := New(cfg, "localhost", 8080)

			Expect(srv.config).NotTo(BeNil())
			Expect(srv.httpServer).NotTo(BeNil())
			Expect(srv.host).To(Equal("localhost"))
			Expect(srv.port).To(Equal(8080))
			Expect(srv.httpServer.ReadTimeout).To(Equal(15 * time.Second))
			Expect(srv.httpServer.WriteTimeout).To(Equal(15 * time.Second))
			Expect(srv.httpServer.IdleTimeout).To(Equal(60 * time.Second))
		})
	})
})
