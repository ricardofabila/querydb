package dynamodb

import (
	"context"
	"flag"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"querydb/internal/dynamodb/testutil"
)

// isShortMode checks if the -test.short flag was passed
func isShortMode() bool {
	f := flag.Lookup("test.short")
	return f != nil && f.Value.String() == "true"
}

// isDynamoDBAvailable checks if DynamoDB Local is reachable on port 8000
func isDynamoDBAvailable() bool {
	conn, err := net.DialTimeout("tcp", "localhost:8000", 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// extractKey extracts the key attributes from an item based on the table name
func extractKey(item map[string]interface{}, tableName string) map[string]interface{} {
	key := make(map[string]interface{})
	switch tableName {
	case "Users", "Orders", "EdgeCases":
		if v, ok := item["userId"]; ok {
			key["userId"] = v
		}
		if v, ok := item["orderId"]; ok {
			key["orderId"] = v
		}
		if v, ok := item["id"]; ok {
			key["id"] = v
		}
	case "Products":
		if v, ok := item["productId"]; ok {
			key["productId"] = v
		}
		if v, ok := item["category"]; ok {
			key["category"] = v
		}
	}
	return key
}

// CRUD integration tests — assumes DynamoDB Local is already running and seeded
// (e.g. via `just test-full` which runs db-up + db-seed before tests)
var _ = Describe("Integration: CRUD Operations", Ordered, func() {
	var client Client

	BeforeAll(func() {
		if isShortMode() {
			Skip("Skipping integration test in short mode")
		}
		if !isDynamoDBAvailable() {
			Skip("DynamoDB Local not available on localhost:8000")
		}

		// Ensure tables exist and are seeded
		ctx := context.Background()
		_ = testutil.SeedTestData(ctx)

		var err error
		client, err = NewClient(
			testutil.DynamoDBLocalEndpoint,
			testutil.DynamoDBLocalRegion,
			testutil.DynamoDBLocalAccessKey,
			testutil.DynamoDBLocalSecretKey,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if client != nil {
			client.Close()
		}
	})

	Describe("PutItem_VariousAttributeTypes", func() {
		DescribeTable("should put items with various attribute types",
			func(tableName string, item map[string]interface{}) {
				err := client.PutItem(tableName, item)
				Expect(err).NotTo(HaveOccurred())

				key := extractKey(item, tableName)
				retrievedItem, err := client.GetItem(tableName, key)
				Expect(err).NotTo(HaveOccurred())
				Expect(retrievedItem).NotTo(BeNil())

				for k, v := range key {
					Expect(retrievedItem[k]).To(Equal(v))
				}
			},
			Entry("simple string and number attributes", "Users", map[string]interface{}{
				"userId": "integration-test-001", "username": "test_user", "email": "test@example.com", "age": int64(28), "isActive": true,
			}),
			Entry("nested map structure", "Users", map[string]interface{}{
				"userId": "integration-test-002", "username": "nested_user", "email": "nested@example.com", "age": int64(35), "isActive": true,
				"profile": map[string]interface{}{"firstName": "Integration", "lastName": "Test", "address": map[string]interface{}{"street": "456 Test Ave", "city": "TestCity", "zipCode": "12345"}},
			}),
			Entry("list attributes", "Users", map[string]interface{}{
				"userId": "integration-test-003", "username": "list_user", "email": "list@example.com", "age": int64(40), "isActive": false, "tags": []interface{}{"integration", "test", "crud"},
			}),
			Entry("null and empty values", "Users", map[string]interface{}{
				"userId": "integration-test-004", "username": "null_user", "email": "", "age": int64(0), "isActive": false, "description": nil,
			}),
			Entry("composite key table", "Products", map[string]interface{}{
				"productId": "integration-prod-001", "category": "test-category", "name": "Test Product", "price": float64(99.99), "inStock": true,
			}),
			Entry("mixed data types", "EdgeCases", map[string]interface{}{
				"id": "integration-edge-001", "emptyString": "", "nullValue": nil, "zeroNumber": int64(0), "negativeNumber": int64(-999),
				"largeNumber": int64(9007199254740991), "decimalNumber": float64(3.14159), "boolTrue": true, "boolFalse": false,
				"emptyList": []interface{}{}, "emptyMap": map[string]interface{}{},
			}),
		)
	})

	Describe("UpdateItem_DifferentPatterns", func() {
		BeforeEach(func() {
			err := client.PutItem("Users", map[string]interface{}{
				"userId": "update-test-001", "username": "update_user", "email": "update@example.com", "age": int64(30), "isActive": true,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("should update items with different patterns",
			func(key map[string]interface{}, updates map[string]interface{}) {
				err := client.UpdateItem("Users", key, updates)
				Expect(err).NotTo(HaveOccurred())

				retrievedItem, err := client.GetItem("Users", key)
				Expect(err).NotTo(HaveOccurred())
				Expect(retrievedItem).NotTo(BeNil())

				for k, expectedValue := range updates {
					if expectedValue == nil {
						Expect(retrievedItem[k]).To(BeNil())
					} else {
						Expect(retrievedItem[k]).To(Equal(expectedValue))
					}
				}
			},
			Entry("update single attribute", map[string]interface{}{"userId": "update-test-001"}, map[string]interface{}{"age": int64(31)}),
			Entry("update multiple attributes", map[string]interface{}{"userId": "update-test-001"}, map[string]interface{}{"username": "updated_user", "email": "updated@example.com", "isActive": false}),
			Entry("update with nested structure", map[string]interface{}{"userId": "update-test-001"}, map[string]interface{}{"profile": map[string]interface{}{"firstName": "Updated", "lastName": "User"}}),
			Entry("update with list", map[string]interface{}{"userId": "update-test-001"}, map[string]interface{}{"tags": []interface{}{"updated", "tags"}}),
			Entry("update with null value", map[string]interface{}{"userId": "update-test-001"}, map[string]interface{}{"description": nil}),
		)

		It("should update composite key table", func() {
			Expect(client.PutItem("Products", map[string]interface{}{
				"productId": "update-prod-001", "category": "update-category", "name": "Original Name", "price": float64(50.00), "inStock": true,
			})).To(Succeed())

			key := map[string]interface{}{"productId": "update-prod-001", "category": "update-category"}
			Expect(client.UpdateItem("Products", key, map[string]interface{}{"name": "Updated Name", "price": float64(75.00), "inStock": false})).To(Succeed())

			retrievedItem, err := client.GetItem("Products", key)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedItem["name"]).To(Equal("Updated Name"))
			price := retrievedItem["price"]
			if priceInt, ok := price.(int64); ok {
				Expect(priceInt).To(Equal(int64(75)))
			} else {
				Expect(price).To(Equal(float64(75.00)))
			}
			Expect(retrievedItem["inStock"]).To(Equal(false))
		})
	})

	Describe("DeleteItem_CompositeKeys", func() {
		DescribeTable("should delete items with various key types",
			func(tableName string, item map[string]interface{}, key map[string]interface{}) {
				Expect(client.PutItem(tableName, item)).To(Succeed())

				retrievedItem, err := client.GetItem(tableName, key)
				Expect(err).NotTo(HaveOccurred())
				Expect(retrievedItem).NotTo(BeNil())

				Expect(client.DeleteItem(tableName, key)).To(Succeed())

				_, err = client.GetItem(tableName, key)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Item not found"))
			},
			Entry("delete with simple key", "Users",
				map[string]interface{}{"userId": "delete-test-001", "username": "delete_user", "email": "delete@example.com", "age": int64(25), "isActive": true},
				map[string]interface{}{"userId": "delete-test-001"}),
			Entry("delete with composite key", "Products",
				map[string]interface{}{"productId": "delete-prod-001", "category": "delete-category", "name": "Delete Test Product", "price": float64(25.00), "inStock": true},
				map[string]interface{}{"productId": "delete-prod-001", "category": "delete-category"}),
			Entry("delete with composite key - different category", "Products",
				map[string]interface{}{"productId": "delete-prod-002", "category": "another-category", "name": "Another Delete Test", "price": float64(30.00), "inStock": false},
				map[string]interface{}{"productId": "delete-prod-002", "category": "another-category"}),
		)
	})

	Describe("GetItem_ExistingAndNonExisting", func() {
		It("should get existing items", func() {
			Expect(client.PutItem("Users", map[string]interface{}{"userId": "get-test-001", "username": "get_user", "email": "get@example.com", "age": int64(27), "isActive": true})).To(Succeed())
			Expect(client.PutItem("Products", map[string]interface{}{"productId": "get-prod-001", "category": "get-category", "name": "Get Test Product", "price": float64(45.00), "inStock": true})).To(Succeed())

			item, err := client.GetItem("Users", map[string]interface{}{"userId": "get-test-001"})
			Expect(err).NotTo(HaveOccurred())
			Expect(item["userId"]).To(Equal("get-test-001"))

			item, err = client.GetItem("Products", map[string]interface{}{"productId": "get-prod-001", "category": "get-category"})
			Expect(err).NotTo(HaveOccurred())
			Expect(item["productId"]).To(Equal("get-prod-001"))
		})

		It("should fail for non-existing simple key", func() {
			_, err := client.GetItem("Users", map[string]interface{}{"userId": "non-existing-user"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Item not found"))
		})

		It("should fail for non-existing composite key", func() {
			_, err := client.GetItem("Products", map[string]interface{}{"productId": "non-existing-product", "category": "non-existing-category"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Item not found"))
		})
	})

	Describe("ErrorHandling_AllOperations", func() {
		It("should fail PutItem for non-existent table", func() {
			err := client.PutItem("NonExistentTable", map[string]interface{}{"id": "test", "name": "test"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should fail UpdateItem for non-existent table", func() {
			err := client.UpdateItem("NonExistentTable", map[string]interface{}{"id": "test"}, map[string]interface{}{"name": "updated"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should fail UpdateItem with empty updates", func() {
			err := client.UpdateItem("Users", map[string]interface{}{"userId": "test"}, map[string]interface{}{})
			Expect(err).To(HaveOccurred())
		})

		It("should fail DeleteItem for non-existent table", func() {
			err := client.DeleteItem("NonExistentTable", map[string]interface{}{"id": "test"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should fail GetItem for non-existent table", func() {
			_, err := client.GetItem("NonExistentTable", map[string]interface{}{"id": "test"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})
})

// Lifecycle tests — test the testutil package itself, run separately via -run Lifecycle
var _ = Describe("Integration: DynamoDB Local Lifecycle", Label("lifecycle"), Ordered, func() {
	BeforeAll(func() {
		if isShortMode() {
			Skip("Skipping integration test in short mode")
		}
	})

	ctx := context.Background()

	It("should start and cleanup successfully", func() {
		Expect(testutil.StartDynamoDBLocal(ctx)).To(Succeed())
		Expect(testutil.IsDynamoDBReady(ctx)).To(BeTrue())
		// Don't call CleanupDynamoDBLocal here — it stops the container
		// and breaks other tests that depend on it
	})

	It("should handle multiple starts idempotently", func() {
		Expect(testutil.StartDynamoDBLocal(ctx)).To(Succeed())
		Expect(testutil.StartDynamoDBLocal(ctx)).To(Succeed())
		Expect(testutil.IsDynamoDBReady(ctx)).To(BeTrue())
	})
})

var _ = Describe("Integration: Seed Data", Label("lifecycle"), Ordered, func() {
	BeforeAll(func() {
		if isShortMode() {
			Skip("Skipping integration test in short mode")
		}
		ctx := context.Background()
		Expect(testutil.StartDynamoDBLocal(ctx)).To(Succeed())
	})

	It("should seed and verify data", func() {
		ctx := context.Background()
		Expect(testutil.SeedTestData(ctx)).To(Succeed())

		verifyClient, err := NewClient(testutil.DynamoDBLocalEndpoint, testutil.DynamoDBLocalRegion, testutil.DynamoDBLocalAccessKey, testutil.DynamoDBLocalSecretKey)
		Expect(err).NotTo(HaveOccurred())
		defer verifyClient.Close()

		for _, table := range []string{"Users", "Products", "Orders", "EdgeCases"} {
			items, err := verifyClient.ScanTable(table)
			Expect(err).NotTo(HaveOccurred())
			Expect(items).NotTo(BeEmpty())
		}
	})

	It("should clean test data", func() {
		ctx := context.Background()
		Expect(testutil.SeedTestData(ctx)).To(Succeed())
		Expect(testutil.CleanTestData(ctx)).To(Succeed())

		verifyClient, err := NewClient(testutil.DynamoDBLocalEndpoint, testutil.DynamoDBLocalRegion, testutil.DynamoDBLocalAccessKey, testutil.DynamoDBLocalSecretKey)
		Expect(err).NotTo(HaveOccurred())
		defer verifyClient.Close()

		_, err = verifyClient.ScanTable("Users")
		Expect(err).To(HaveOccurred())
		_, err = verifyClient.ScanTable("Products")
		Expect(err).To(HaveOccurred())

		// Re-seed so other tests aren't affected
		Expect(testutil.SeedTestData(ctx)).To(Succeed())
	})
})
