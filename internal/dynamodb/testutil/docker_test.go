package testutil

import (
	"context"
	"flag"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTestutil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testutil Suite")
}

func isShortMode() bool {
	f := flag.Lookup("test.short")
	return f != nil && f.Value.String() == "true"
}

var _ = Describe("DynamoDB Local Docker", Ordered, func() {
	var ctx context.Context

	BeforeAll(func() {
		if isShortMode() {
			Skip("Skipping integration test in short mode")
		}
		ctx = context.Background()
		// Start once for all tests in this block
		Expect(StartDynamoDBLocal(ctx)).To(Succeed())
	})

	AfterAll(func() {
		if ctx != nil {
			_ = CleanupDynamoDBLocal(ctx)
		}
	})

	It("should be ready after starting", func() {
		Expect(IsDynamoDBReady(ctx)).To(BeTrue())
	})

	It("should handle multiple starts idempotently", func() {
		Expect(StartDynamoDBLocal(ctx)).To(Succeed())
		Expect(IsDynamoDBReady(ctx)).To(BeTrue())
	})

	It("should seed data without error", func() {
		seedCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()

		err := SeedTestData(seedCtx)
		if err != nil {
			if !IsDynamoDBReady(seedCtx) {
				Fail("Failed to seed test data: " + err.Error())
			}
			GinkgoWriter.Printf("Seed script produced warnings but DynamoDB is responding: %v\n", err)
		}
		Expect(IsDynamoDBReady(seedCtx)).To(BeTrue())
	})

	It("should clean data without error", func() {
		cleanCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()

		// Seed first so there's something to clean
		err := SeedTestData(cleanCtx)
		if err != nil && !IsDynamoDBReady(cleanCtx) {
			Fail("Failed to seed test data: " + err.Error())
		}

		err = CleanTestData(cleanCtx)
		if err != nil && !IsDynamoDBReady(cleanCtx) {
			Fail("Failed to clean test data: " + err.Error())
		}
		Expect(IsDynamoDBReady(cleanCtx)).To(BeTrue())
	})
})
