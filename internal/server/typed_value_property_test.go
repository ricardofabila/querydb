package server

import (
	"encoding/base64"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	sdkdynamodb "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Validates: Requirements 20.4**
// Property 2: Typed value round-trip consistency

// genScalarAV generates a random scalar DynamoDB AttributeValue (S, N, BOOL, NULL, B).
func genScalarAV() gopter.Gen {
	return gen.OneGenOf(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }).Map(
			func(s string) *sdkdynamodb.AttributeValue {
				return &sdkdynamodb.AttributeValue{S: aws.String(s)}
			},
		),
		gen.Int64Range(-99999, 99999).Map(func(n int64) *sdkdynamodb.AttributeValue {
			return &sdkdynamodb.AttributeValue{N: aws.String(fmt.Sprintf("%d", n))}
		}),
		gen.Bool().Map(func(b bool) *sdkdynamodb.AttributeValue {
			return &sdkdynamodb.AttributeValue{BOOL: aws.Bool(b)}
		}),
		gen.Const(&sdkdynamodb.AttributeValue{NULL: aws.Bool(true)}),
		gen.SliceOfN(8, gen.UInt8Range(0, 255)).SuchThat(func(b []uint8) bool { return len(b) > 0 }).Map(
			func(b []uint8) *sdkdynamodb.AttributeValue {
				bs := make([]byte, len(b))
				for i, v := range b {
					bs[i] = byte(v)
				}
				return &sdkdynamodb.AttributeValue{B: bs}
			},
		),
	)
}

// genSetAV generates a random set-type DynamoDB AttributeValue (SS, NS, BS).
func genSetAV() gopter.Gen {
	return gen.OneGenOf(
		gen.SliceOfN(3, gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 })).
			SuchThat(func(ss []string) bool { return len(ss) > 0 }).
			Map(func(ss []string) *sdkdynamodb.AttributeValue {
				seen := map[string]bool{}
				ptrs := make([]*string, 0, len(ss))
				for _, s := range ss {
					if !seen[s] {
						seen[s] = true
						ptrs = append(ptrs, aws.String(s))
					}
				}
				return &sdkdynamodb.AttributeValue{SS: ptrs}
			}),
		gen.SliceOfN(3, gen.Int64Range(-9999, 9999)).
			SuchThat(func(ns []int64) bool { return len(ns) > 0 }).
			Map(func(ns []int64) *sdkdynamodb.AttributeValue {
				seen := map[string]bool{}
				ptrs := make([]*string, 0, len(ns))
				for _, n := range ns {
					s := fmt.Sprintf("%d", n)
					if !seen[s] {
						seen[s] = true
						ptrs = append(ptrs, aws.String(s))
					}
				}
				return &sdkdynamodb.AttributeValue{NS: ptrs}
			}),
		gen.SliceOfN(3, gen.SliceOfN(4, gen.UInt8Range(0, 255))).
			SuchThat(func(bs [][]uint8) bool { return len(bs) > 0 }).
			Map(func(bs [][]uint8) *sdkdynamodb.AttributeValue {
				result := make([][]byte, len(bs))
				for i, b := range bs {
					buf := make([]byte, len(b))
					for j, v := range b {
						buf[j] = byte(v)
					}
					result[i] = buf
				}
				return &sdkdynamodb.AttributeValue{BS: result}
			}),
	)
}

// genSimpleMapAV generates a Map AV with 1-3 scalar entries.
func genSimpleMapAV() gopter.Gen {
	return gen.IntRange(1, 3).FlatMap(func(v interface{}) gopter.Gen {
		n := v.(int)
		return gen.SliceOfN(n, gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 })).FlatMap(func(v interface{}) gopter.Gen {
			keys := v.([]string)
			seen := map[string]bool{}
			unique := make([]string, 0, len(keys))
			for _, k := range keys {
				if !seen[k] {
					seen[k] = true
					unique = append(unique, k)
				}
			}
			return gen.SliceOfN(len(unique), genScalarAV()).Map(
				func(vals []*sdkdynamodb.AttributeValue) *sdkdynamodb.AttributeValue {
					m := make(map[string]*sdkdynamodb.AttributeValue)
					for i, k := range unique {
						if i < len(vals) {
							m[k] = vals[i]
						}
					}
					return &sdkdynamodb.AttributeValue{M: m}
				},
			)
		}, reflect.TypeOf((*sdkdynamodb.AttributeValue)(nil)))
	}, reflect.TypeOf((*sdkdynamodb.AttributeValue)(nil)))
}

// genSimpleListAV generates a List AV with 1-3 scalar entries.
func genSimpleListAV() gopter.Gen {
	return gen.IntRange(1, 3).FlatMap(func(v interface{}) gopter.Gen {
		n := v.(int)
		return gen.SliceOfN(n, genScalarAV()).Map(
			func(vals []*sdkdynamodb.AttributeValue) *sdkdynamodb.AttributeValue {
				return &sdkdynamodb.AttributeValue{L: vals}
			},
		)
	}, reflect.TypeOf((*sdkdynamodb.AttributeValue)(nil)))
}

// genAnyAV generates a random DynamoDB AttributeValue of any supported type.
func genAnyAV() gopter.Gen {
	return gen.OneGenOf(
		genScalarAV(),
		genSetAV(),
		genSimpleMapAV(),
		genSimpleListAV(),
	)
}

// avEqual compares two AttributeValues for structural equality.
func avEqual(a, b *sdkdynamodb.AttributeValue) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// S
	if (a.S != nil) != (b.S != nil) {
		return false
	}
	if a.S != nil && *a.S != *b.S {
		return false
	}

	// N
	if (a.N != nil) != (b.N != nil) {
		return false
	}
	if a.N != nil && *a.N != *b.N {
		return false
	}

	// BOOL
	if (a.BOOL != nil) != (b.BOOL != nil) {
		return false
	}
	if a.BOOL != nil && *a.BOOL != *b.BOOL {
		return false
	}

	// NULL
	if (a.NULL != nil) != (b.NULL != nil) {
		return false
	}
	if a.NULL != nil && *a.NULL != *b.NULL {
		return false
	}

	// B
	if (a.B != nil) != (b.B != nil) {
		return false
	}
	if a.B != nil {
		if base64.StdEncoding.EncodeToString(a.B) != base64.StdEncoding.EncodeToString(b.B) {
			return false
		}
	}

	// SS
	if (a.SS != nil) != (b.SS != nil) {
		return false
	}
	if a.SS != nil {
		if len(a.SS) != len(b.SS) {
			return false
		}
		for i := range a.SS {
			if *a.SS[i] != *b.SS[i] {
				return false
			}
		}
	}

	// NS
	if (a.NS != nil) != (b.NS != nil) {
		return false
	}
	if a.NS != nil {
		if len(a.NS) != len(b.NS) {
			return false
		}
		for i := range a.NS {
			if *a.NS[i] != *b.NS[i] {
				return false
			}
		}
	}

	// BS
	if (a.BS != nil) != (b.BS != nil) {
		return false
	}
	if a.BS != nil {
		if len(a.BS) != len(b.BS) {
			return false
		}
		for i := range a.BS {
			if base64.StdEncoding.EncodeToString(a.BS[i]) != base64.StdEncoding.EncodeToString(b.BS[i]) {
				return false
			}
		}
	}

	// M
	if (a.M != nil) != (b.M != nil) {
		return false
	}
	if a.M != nil {
		if len(a.M) != len(b.M) {
			return false
		}
		for k, av := range a.M {
			bv, ok := b.M[k]
			if !ok {
				return false
			}
			if !avEqual(av, bv) {
				return false
			}
		}
	}

	// L
	if (a.L != nil) != (b.L != nil) {
		return false
	}
	if a.L != nil {
		if len(a.L) != len(b.L) {
			return false
		}
		for i := range a.L {
			if !avEqual(a.L[i], b.L[i]) {
				return false
			}
		}
	}

	return true
}

var _ = Describe("TypedValue Property Tests", func() {
	It("AV → TypedValue → AV round-trip produces equivalent AttributeValue", func() {
		parameters := gopter.DefaultTestParameters()
		parameters.MinSuccessfulTests = 50

		properties := gopter.NewProperties(parameters)

		properties.Property("AV → TypedValue → AV round-trip produces equivalent AttributeValue",
			prop.ForAll(
				func(original *sdkdynamodb.AttributeValue) bool {
					tv := convertAVToTyped(original)

					roundTripped, err := convertTypedToAV(tv)
					if err != nil {
						return false
					}

					return avEqual(original, roundTripped)
				},
				genAnyAV(),
			))

		Expect(properties.Run(gopter.ConsoleReporter(false))).To(BeTrue())
	})
})
