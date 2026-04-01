package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"querydb/internal/config"
	"querydb/internal/dynamodb"
)

const (
	integrationEndpoint  = "http://localhost:8000"
	integrationRegion    = "us-west-2"
	integrationAccessKey = "foo"
	integrationSecretKey = "bar"
)

func skipIfNoDynamoDBLocal() {
	if CurrentSpecReport().IsSerial {
		// placeholder
	}
	conn, err := net.DialTimeout("tcp", "localhost:8000", 2*time.Second)
	if err != nil {
		Skip("requires DynamoDB Local: could not connect to localhost:8000")
	}
	conn.Close()
}

func integrationConfig(configName, tableName string) *config.Config {
	return &config.Config{
		Tables: map[string]config.TableConfig{
			configName: {
				TableName:       tableName,
				Endpoint:        integrationEndpoint,
				Region:          integrationRegion,
				AccessKeyID:     integrationAccessKey,
				SecretAccessKey: integrationSecretKey,
			},
		},
	}
}

func integrationClient() dynamodb.Client {
	client, err := dynamodb.NewClient(integrationEndpoint, integrationRegion, integrationAccessKey, integrationSecretKey)
	Expect(err).NotTo(HaveOccurred(), "Failed to create DynamoDB client for integration test")
	return client
}

func ensureTestTable(client dynamodb.Client) {
	_, err := client.ScanTable("TestTable")
	if err == nil {
		return
	}
	Skip("requires TestTable to exist in DynamoDB Local (run test/seed_data.sh)")
}

var _ = Describe("Handlers Integration", func() {
	// Property 5: Table Data Retrieval
	// **Validates: Requirements 3.3, 4.1, 4.4, 10.2**
	Describe("HandleGetItems", func() {
		It("retrieves table data through the handler", func() {
			skipIfNoDynamoDBLocal()

			client := integrationClient()
			defer client.Close()

			_, err := client.ScanTable("Users")
			if err != nil {
				Skip("requires Users table in DynamoDB Local (run test/seed_data.sh)")
			}

			cfg := integrationConfig("test-users", "Users")
			srv := New(cfg, "127.0.0.1", 8080)

			req := httptest.NewRequest(http.MethodGet, "/api/tables/test-users/items", nil)
			w := httptest.NewRecorder()

			srv.handleGetItems(w, req, "test-users")

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&resp)).To(Succeed())
			Expect(resp.Success).To(BeTrue())

			dataBytes, err := json.Marshal(resp.Data)
			Expect(err).NotTo(HaveOccurred())

			var itemsResp ItemsResponse
			Expect(json.Unmarshal(dataBytes, &itemsResp)).To(Succeed())

			Expect(itemsResp.Count).To(BeNumerically(">=", 1), "Expected at least 1 item from Users table")
			Expect(itemsResp.Items).To(HaveLen(itemsResp.Count))

			for _, item := range itemsResp.Items {
				userId, ok := item["userId"]
				Expect(ok).To(BeTrue(), "Each user item should have a userId attribute")
				_, isString := userId.(string)
				Expect(isString).To(BeTrue(), "userId should be a string type")
			}
		})
	})

	// Property 6: Item Creation via API
	// **Validates: Requirements 6.4, 10.3**
	Describe("HandleCreateItem", func() {
		It("creates an item through the handler", func() {
			skipIfNoDynamoDBLocal()

			client := integrationClient()
			defer client.Close()

			_, err := client.ScanTable("Users")
			if err != nil {
				Skip("requires Users table in DynamoDB Local (run test/seed_data.sh)")
			}

			cfg := integrationConfig("test-users", "Users")
			srv := New(cfg, "127.0.0.1", 8080)

			newItem := map[string]interface{}{
				"userId":   "integration-handler-create-001",
				"username": "handler_test_user",
				"email":    "handler@test.com",
				"age":      float64(33),
				"isActive": true,
			}

			body, err := json.Marshal(CreateItemRequest{Item: newItem})
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodPost, "/api/tables/test-users/items", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleCreateItem(w, req, "test-users")

			Expect(w.Code).To(Equal(http.StatusOK))

			var createResp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&createResp)).To(Succeed())
			Expect(createResp.Success).To(BeTrue())

			// Verify the item was created by fetching items through the handler
			getReq := httptest.NewRequest(http.MethodGet, "/api/tables/test-users/items", nil)
			getW := httptest.NewRecorder()

			srv.handleGetItems(getW, getReq, "test-users")

			Expect(getW.Code).To(Equal(http.StatusOK))

			var getResp APIResponse
			Expect(json.NewDecoder(getW.Body).Decode(&getResp)).To(Succeed())

			dataBytes, _ := json.Marshal(getResp.Data)
			var itemsResp ItemsResponse
			Expect(json.Unmarshal(dataBytes, &itemsResp)).To(Succeed())

			found := false
			for _, item := range itemsResp.Items {
				if item["userId"] == "integration-handler-create-001" {
					found = true
					Expect(item["username"]).To(Equal("handler_test_user"))
					Expect(item["email"]).To(Equal("handler@test.com"))
					break
				}
			}
			Expect(found).To(BeTrue(), "Created item should appear in GetItems response")

			// Cleanup
			_ = client.DeleteItem("Users", map[string]interface{}{"userId": "integration-handler-create-001"})
		})
	})

	// Property 7: Item Update via API
	// **Validates: Requirements 7.4, 10.4**
	Describe("HandleUpdateItem", func() {
		It("updates an item through the handler", func() {
			skipIfNoDynamoDBLocal()

			client := integrationClient()
			defer client.Close()

			_, err := client.ScanTable("Users")
			if err != nil {
				Skip("requires Users table in DynamoDB Local (run test/seed_data.sh)")
			}

			cfg := integrationConfig("test-users", "Users")
			srv := New(cfg, "127.0.0.1", 8080)

			seedItem := map[string]interface{}{
				"userId":   "integration-handler-update-001",
				"username": "before_update",
				"email":    "before@test.com",
				"age":      float64(20),
				"isActive": true,
			}
			Expect(client.PutItem("Users", seedItem)).To(Succeed())

			updates := UpdateItemRequest{
				Updates: map[string]interface{}{
					"username": "after_update",
					"email":    "after@test.com",
					"age":      float64(21),
				},
			}

			body, err := json.Marshal(updates)
			Expect(err).NotTo(HaveOccurred())

			keyJSON := `{"userId":"integration-handler-update-001"}`
			encodedKey := url.PathEscape(keyJSON)

			reqPath := fmt.Sprintf("/api/tables/test-users/items/%s", encodedKey)
			req := httptest.NewRequest(http.MethodPut, reqPath, bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			srv.handleUpdateItem(w, req, "test-users", keyJSON)

			Expect(w.Code).To(Equal(http.StatusOK))

			var updateResp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&updateResp)).To(Succeed())
			Expect(updateResp.Success).To(BeTrue())

			updatedItem, err := client.GetItem("Users", map[string]interface{}{"userId": "integration-handler-update-001"})
			Expect(err).NotTo(HaveOccurred())

			Expect(updatedItem["username"]).To(Equal("after_update"))
			Expect(updatedItem["email"]).To(Equal("after@test.com"))

			// Cleanup
			_ = client.DeleteItem("Users", map[string]interface{}{"userId": "integration-handler-update-001"})
		})
	})

	// Property 8: Item Deletion via API
	// **Validates: Requirements 8.3, 10.5**
	Describe("HandleDeleteItem", func() {
		It("deletes an item through the handler", func() {
			skipIfNoDynamoDBLocal()

			client := integrationClient()
			defer client.Close()

			_, err := client.ScanTable("Users")
			if err != nil {
				Skip("requires Users table in DynamoDB Local (run test/seed_data.sh)")
			}

			cfg := integrationConfig("test-users", "Users")
			srv := New(cfg, "127.0.0.1", 8080)

			seedItem := map[string]interface{}{
				"userId":   "integration-handler-delete-001",
				"username": "to_be_deleted",
				"email":    "delete@test.com",
			}
			Expect(client.PutItem("Users", seedItem)).To(Succeed())

			existingItem, err := client.GetItem("Users", map[string]interface{}{"userId": "integration-handler-delete-001"})
			Expect(err).NotTo(HaveOccurred())
			Expect(existingItem).NotTo(BeNil())

			keyJSON := `{"userId":"integration-handler-delete-001"}`
			encodedKey := url.PathEscape(keyJSON)

			reqPath := fmt.Sprintf("/api/tables/test-users/items/%s", encodedKey)
			req := httptest.NewRequest(http.MethodDelete, reqPath, nil)
			w := httptest.NewRecorder()

			srv.handleDeleteItem(w, req, "test-users", keyJSON)

			Expect(w.Code).To(Equal(http.StatusOK))

			var deleteResp APIResponse
			Expect(json.NewDecoder(w.Body).Decode(&deleteResp)).To(Succeed())
			Expect(deleteResp.Success).To(BeTrue())

			_, err = client.GetItem("Users", map[string]interface{}{"userId": "integration-handler-delete-001"})
			Expect(err).To(HaveOccurred(), "Deleted item should not be retrievable")
			Expect(err.Error()).To(ContainSubstring("Item not found"))

			getReq := httptest.NewRequest(http.MethodGet, "/api/tables/test-users/items", nil)
			getW := httptest.NewRecorder()

			srv.handleGetItems(getW, getReq, "test-users")

			Expect(getW.Code).To(Equal(http.StatusOK))

			var getResp APIResponse
			Expect(json.NewDecoder(getW.Body).Decode(&getResp)).To(Succeed())

			dataBytes, _ := json.Marshal(getResp.Data)
			var itemsResp ItemsResponse
			_ = json.Unmarshal(dataBytes, &itemsResp)

			for _, item := range itemsResp.Items {
				Expect(item["userId"]).NotTo(Equal("integration-handler-delete-001"),
					"Deleted item should not appear in scan results")
			}
		})
	})
})
