package dynamodb

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	sdkdynamodb "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// **Validates: Requirements 19.7**
// Property 1: QueryTable round-trip consistency
// For any valid QueryResult containing DynamoDB items, serializing the result
// to JSON and deserializing it back should produce an equivalent QueryResult
// for the metadata fields (Count, ScannedCount). Items are tested with simple
// types that serialize cleanly through the AWS SDK's JSON representation.

// genSimpleAttributeValue generates a random DynamoDB AttributeValue of a simple type (S, N, BOOL, NULL).
func genSimpleAttributeValue() gopter.Gen {
	return gen.OneGenOf(
		gen.AlphaString().Map(func(s string) *sdkdynamodb.AttributeValue {
			return &sdkdynamodb.AttributeValue{S: aws.String(s)}
		}),
		gen.Int64Range(-999999, 999999).Map(func(n int64) *sdkdynamodb.AttributeValue {
			return &sdkdynamodb.AttributeValue{N: aws.String(fmt.Sprintf("%d", n))}
		}),
		gen.Bool().Map(func(b bool) *sdkdynamodb.AttributeValue {
			return &sdkdynamodb.AttributeValue{BOOL: aws.Bool(b)}
		}),
		gen.Const(&sdkdynamodb.AttributeValue{NULL: aws.Bool(true)}),
	)
}

// genDynamoDBItem generates a random DynamoDB item with 1-5 attributes of simple types.
func genDynamoDBItem() gopter.Gen {
	return gen.IntRange(1, 5).FlatMap(func(v interface{}) gopter.Gen {
		n := v.(int)
		return gen.SliceOfN(n, gen.AlphaString()).FlatMap(func(v interface{}) gopter.Gen {
			keys := v.([]string)
			// Deduplicate keys
			seen := make(map[string]bool)
			unique := make([]string, 0, len(keys))
			for _, k := range keys {
				if k == "" {
					k = "attr"
				}
				if !seen[k] {
					seen[k] = true
					unique = append(unique, k)
				}
			}
			if len(unique) == 0 {
				unique = []string{"pk"}
			}
			return gen.SliceOfN(len(unique), genSimpleAttributeValue()).Map(
				func(vals []*sdkdynamodb.AttributeValue) map[string]*sdkdynamodb.AttributeValue {
					item := make(map[string]*sdkdynamodb.AttributeValue)
					for i, k := range unique {
						if i < len(vals) {
							item[k] = vals[i]
						}
					}
					return item
				},
			)
		}, reflect.TypeOf(map[string]*sdkdynamodb.AttributeValue{}))
	}, reflect.TypeOf(map[string]*sdkdynamodb.AttributeValue{}))
}

// genQueryResult generates a random QueryResult with metadata and 0-5 items.
func genQueryResult() gopter.Gen {
	return gen.IntRange(0, 5).FlatMap(func(v interface{}) gopter.Gen {
		numItems := v.(int)
		return gen.SliceOfN(numItems, genDynamoDBItem()).FlatMap(func(v interface{}) gopter.Gen {
			items := v.([]map[string]*sdkdynamodb.AttributeValue)
			return gen.Int64Range(0, 1000).FlatMap(func(v interface{}) gopter.Gen {
				scannedCount := v.(int64)
				count := int64(len(items))
				if scannedCount < count {
					scannedCount = count
				}
				result := QueryResult{
					Items:        items,
					Count:        count,
					ScannedCount: scannedCount,
				}
				return gen.Const(result)
			}, reflect.TypeOf(QueryResult{}))
		}, reflect.TypeOf(QueryResult{}))
	}, reflect.TypeOf(QueryResult{}))
}

var _ = Describe("Property: QueryResult Round-Trip", func() {
	It("should preserve QueryResult metadata through JSON serialization", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 50

		properties := gopter.NewProperties(parameters)

		properties.Property("QueryResult metadata round-trips through JSON serialization",
			prop.ForAll(
				func(original QueryResult) bool {
					// Marshal to JSON
					data, err := json.Marshal(original)
					if err != nil {
						GinkgoWriter.Printf("Failed to marshal: %v\n", err)
						return false
					}

					// Unmarshal back
					var roundTripped QueryResult
					err = json.Unmarshal(data, &roundTripped)
					if err != nil {
						GinkgoWriter.Printf("Failed to unmarshal: %v\n", err)
						return false
					}

					// Verify metadata fields are preserved
					if original.Count != roundTripped.Count {
						GinkgoWriter.Printf("Count mismatch: original=%d, roundTripped=%d\n", original.Count, roundTripped.Count)
						return false
					}
					if original.ScannedCount != roundTripped.ScannedCount {
						GinkgoWriter.Printf("ScannedCount mismatch: original=%d, roundTripped=%d\n", original.ScannedCount, roundTripped.ScannedCount)
						return false
					}

					// Verify item count is preserved
					if len(original.Items) != len(roundTripped.Items) {
						GinkgoWriter.Printf("Items length mismatch: original=%d, roundTripped=%d\n", len(original.Items), len(roundTripped.Items))
						return false
					}

					// Verify each item's attribute keys are preserved
					for i, origItem := range original.Items {
						rtItem := roundTripped.Items[i]
						if len(origItem) != len(rtItem) {
							GinkgoWriter.Printf("Item[%d] attribute count mismatch: original=%d, roundTripped=%d\n", i, len(origItem), len(rtItem))
							return false
						}
						for key := range origItem {
							if _, exists := rtItem[key]; !exists {
								GinkgoWriter.Printf("Item[%d] missing key %q after round-trip\n", i, key)
								return false
							}
						}
					}

					// Verify LastEvaluatedKey is nil in both (we generate without it)
					if (original.LastEvaluatedKey == nil) != (roundTripped.LastEvaluatedKey == nil) {
						GinkgoWriter.Printf("LastEvaluatedKey nil mismatch\n")
						return false
					}

					// Verify ConsumedCapacity is nil in both (we generate without it)
					if (original.ConsumedCapacity == nil) != (roundTripped.ConsumedCapacity == nil) {
						GinkgoWriter.Printf("ConsumedCapacity nil mismatch\n")
						return false
					}

					return true
				},
				genQueryResult(),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})
})
