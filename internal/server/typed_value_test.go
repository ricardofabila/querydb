package server

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	sdkdynamodb "github.com/aws/aws-sdk-go/service/dynamodb"
)

var _ = Describe("TypedValue", func() {
	Describe("convertAVToTyped", func() {
		It("converts S type", func() {
			av := &sdkdynamodb.AttributeValue{S: aws.String("hello")}
			tv := convertAVToTyped(av)
			Expect(tv.Type).To(Equal("S"))
			Expect(tv.Value).To(Equal("hello"))
		})

		It("converts N type", func() {
			av := &sdkdynamodb.AttributeValue{N: aws.String("42.5")}
			tv := convertAVToTyped(av)
			Expect(tv.Type).To(Equal("N"))
			Expect(tv.Value).To(Equal("42.5"))
		})

		It("converts BOOL type", func() {
			av := &sdkdynamodb.AttributeValue{BOOL: aws.Bool(true)}
			tv := convertAVToTyped(av)
			Expect(tv.Type).To(Equal("BOOL"))
			Expect(tv.Value).To(Equal(true))
		})

		It("converts NULL type", func() {
			av := &sdkdynamodb.AttributeValue{NULL: aws.Bool(true)}
			tv := convertAVToTyped(av)
			Expect(tv.Type).To(Equal("NULL"))
			Expect(tv.Value).To(BeNil())
		})

		It("converts M type with nested values", func() {
			av := &sdkdynamodb.AttributeValue{
				M: map[string]*sdkdynamodb.AttributeValue{
					"name":   {S: aws.String("Alice")},
					"age":    {N: aws.String("30")},
					"active": {BOOL: aws.Bool(true)},
					"nested": {M: map[string]*sdkdynamodb.AttributeValue{
						"key": {S: aws.String("deep")},
					}},
				},
			}
			tv := convertAVToTyped(av)

			Expect(tv.Type).To(Equal("M"))

			m, ok := tv.Value.(map[string]TypedValue)
			Expect(ok).To(BeTrue())

			Expect(m["name"].Type).To(Equal("S"))
			Expect(m["name"].Value).To(Equal("Alice"))
			Expect(m["age"].Type).To(Equal("N"))
			Expect(m["age"].Value).To(Equal("30"))
			Expect(m["active"].Type).To(Equal("BOOL"))
			Expect(m["active"].Value).To(Equal(true))

			nested, ok := m["nested"].Value.(map[string]TypedValue)
			Expect(ok).To(BeTrue())
			Expect(nested["key"].Type).To(Equal("S"))
			Expect(nested["key"].Value).To(Equal("deep"))
		})

		It("converts L type with mixed values", func() {
			av := &sdkdynamodb.AttributeValue{
				L: []*sdkdynamodb.AttributeValue{
					{S: aws.String("hello")},
					{N: aws.String("42")},
					{BOOL: aws.Bool(false)},
					{L: []*sdkdynamodb.AttributeValue{
						{S: aws.String("nested")},
					}},
				},
			}
			tv := convertAVToTyped(av)

			Expect(tv.Type).To(Equal("L"))

			l, ok := tv.Value.([]TypedValue)
			Expect(ok).To(BeTrue())
			Expect(l).To(HaveLen(4))
			Expect(l[0].Type).To(Equal("S"))
			Expect(l[0].Value).To(Equal("hello"))
			Expect(l[1].Type).To(Equal("N"))
			Expect(l[1].Value).To(Equal("42"))
			Expect(l[2].Type).To(Equal("BOOL"))
			Expect(l[2].Value).To(Equal(false))

			nestedList, ok := l[3].Value.([]TypedValue)
			Expect(ok).To(BeTrue())
			Expect(nestedList[0].Type).To(Equal("S"))
			Expect(nestedList[0].Value).To(Equal("nested"))
		})

		It("converts SS type", func() {
			av := &sdkdynamodb.AttributeValue{
				SS: []*string{aws.String("a"), aws.String("b"), aws.String("c")},
			}
			tv := convertAVToTyped(av)

			Expect(tv.Type).To(Equal("SS"))

			ss, ok := tv.Value.([]string)
			Expect(ok).To(BeTrue())
			Expect(ss).To(Equal([]string{"a", "b", "c"}))
		})

		It("converts NS type", func() {
			av := &sdkdynamodb.AttributeValue{
				NS: []*string{aws.String("1"), aws.String("2.5"), aws.String("100")},
			}
			tv := convertAVToTyped(av)

			Expect(tv.Type).To(Equal("NS"))

			ns, ok := tv.Value.([]string)
			Expect(ok).To(BeTrue())
			Expect(ns).To(Equal([]string{"1", "2.5", "100"}))
		})

		It("converts BS type (base64 encoded)", func() {
			data1 := []byte("binary1")
			data2 := []byte("binary2")
			av := &sdkdynamodb.AttributeValue{
				BS: [][]byte{data1, data2},
			}
			tv := convertAVToTyped(av)

			Expect(tv.Type).To(Equal("BS"))

			bs, ok := tv.Value.([]string)
			Expect(ok).To(BeTrue())
			Expect(bs).To(HaveLen(2))
			Expect(bs[0]).To(Equal(base64.StdEncoding.EncodeToString(data1)))
			Expect(bs[1]).To(Equal(base64.StdEncoding.EncodeToString(data2)))
		})

		It("converts B type (base64 encoded)", func() {
			data := []byte("hello binary")
			av := &sdkdynamodb.AttributeValue{B: data}
			tv := convertAVToTyped(av)

			Expect(tv.Type).To(Equal("B"))

			encoded, ok := tv.Value.(string)
			Expect(ok).To(BeTrue())
			Expect(encoded).To(Equal(base64.StdEncoding.EncodeToString(data)))
		})

		It("defaults empty AV to NULL", func() {
			av := &sdkdynamodb.AttributeValue{}
			tv := convertAVToTyped(av)

			Expect(tv.Type).To(Equal("NULL"))
			Expect(tv.Value).To(BeNil())
		})
	})

	Describe("convertToTypedItem", func() {
		It("converts full item", func() {
			item := map[string]*sdkdynamodb.AttributeValue{
				"pk":     {S: aws.String("user-123")},
				"sk":     {S: aws.String("PROFILE")},
				"age":    {N: aws.String("25")},
				"active": {BOOL: aws.Bool(true)},
				"data":   {NULL: aws.Bool(true)},
			}

			result := convertToTypedItem(item)

			Expect(result).To(HaveLen(5))

			tests := []struct {
				key      string
				expType  string
				expValue interface{}
			}{
				{"pk", "S", "user-123"},
				{"sk", "S", "PROFILE"},
				{"age", "N", "25"},
				{"active", "BOOL", true},
				{"data", "NULL", nil},
			}

			for _, tt := range tests {
				tv, exists := result[tt.key]
				Expect(exists).To(BeTrue(), "missing key %q in result", tt.key)
				Expect(tv.Type).To(Equal(tt.expType), "key %q", tt.key)
				if tt.expValue == nil {
					Expect(tv.Value).To(BeNil(), "key %q", tt.key)
				} else {
					Expect(tv.Value).To(Equal(tt.expValue), "key %q", tt.key)
				}
			}
		})
	})

	Describe("convertTypedToAV", func() {
		DescribeTable("converts all types correctly",
			func(tv TypedValue, validate func(*sdkdynamodb.AttributeValue)) {
				av, err := convertTypedToAV(tv)
				Expect(err).NotTo(HaveOccurred())
				validate(av)
			},
			Entry("S type", TypedValue{Value: "hello", Type: "S"}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.S).NotTo(BeNil())
				Expect(*av.S).To(Equal("hello"))
			}),
			Entry("N type from string", TypedValue{Value: "42.5", Type: "N"}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.N).NotTo(BeNil())
				Expect(*av.N).To(Equal("42.5"))
			}),
			Entry("N type from float64", TypedValue{Value: float64(99), Type: "N"}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.N).NotTo(BeNil())
				Expect(*av.N).To(Equal("99"))
			}),
			Entry("BOOL type", TypedValue{Value: true, Type: "BOOL"}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.BOOL).NotTo(BeNil())
				Expect(*av.BOOL).To(BeTrue())
			}),
			Entry("NULL type", TypedValue{Value: nil, Type: "NULL"}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.NULL).NotTo(BeNil())
				Expect(*av.NULL).To(BeTrue())
			}),
			Entry("B type", TypedValue{Value: base64.StdEncoding.EncodeToString([]byte("binary")), Type: "B"}, func(av *sdkdynamodb.AttributeValue) {
				Expect(string(av.B)).To(Equal("binary"))
			}),
			Entry("SS type", TypedValue{Value: []string{"a", "b"}, Type: "SS"}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.SS).To(HaveLen(2))
				Expect(*av.SS[0]).To(Equal("a"))
				Expect(*av.SS[1]).To(Equal("b"))
			}),
			Entry("NS type", TypedValue{Value: []string{"1", "2"}, Type: "NS"}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.NS).To(HaveLen(2))
				Expect(*av.NS[0]).To(Equal("1"))
				Expect(*av.NS[1]).To(Equal("2"))
			}),
			Entry("BS type", TypedValue{
				Value: []string{
					base64.StdEncoding.EncodeToString([]byte("bin1")),
					base64.StdEncoding.EncodeToString([]byte("bin2")),
				},
				Type: "BS",
			}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.BS).To(HaveLen(2))
				Expect(string(av.BS[0])).To(Equal("bin1"))
				Expect(string(av.BS[1])).To(Equal("bin2"))
			}),
			Entry("M type with direct Go types", TypedValue{
				Value: map[string]TypedValue{
					"name": {Value: "Alice", Type: "S"},
					"age":  {Value: "30", Type: "N"},
				},
				Type: "M",
			}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.M).NotTo(BeNil())
				Expect(*av.M["name"].S).To(Equal("Alice"))
				Expect(*av.M["age"].N).To(Equal("30"))
			}),
			Entry("L type with direct Go types", TypedValue{
				Value: []TypedValue{
					{Value: "hello", Type: "S"},
					{Value: "42", Type: "N"},
				},
				Type: "L",
			}, func(av *sdkdynamodb.AttributeValue) {
				Expect(av.L).To(HaveLen(2))
				Expect(*av.L[0].S).To(Equal("hello"))
				Expect(*av.L[1].N).To(Equal("42"))
			}),
		)

		DescribeTable("returns error for invalid types",
			func(tv TypedValue) {
				_, err := convertTypedToAV(tv)
				Expect(err).To(HaveOccurred())
			},
			Entry("unsupported type code", TypedValue{Value: "test", Type: "UNKNOWN"}),
			Entry("wrong value type for S", TypedValue{Value: 123, Type: "S"}),
			Entry("wrong value type for BOOL", TypedValue{Value: "notbool", Type: "BOOL"}),
			Entry("wrong value type for N", TypedValue{Value: true, Type: "N"}),
			Entry("wrong value type for B", TypedValue{Value: 42, Type: "B"}),
			Entry("wrong value type for M", TypedValue{Value: "notamap", Type: "M"}),
			Entry("wrong value type for L", TypedValue{Value: "notalist", Type: "L"}),
			Entry("wrong value type for SS", TypedValue{Value: 42, Type: "SS"}),
			Entry("wrong value type for NS", TypedValue{Value: true, Type: "NS"}),
			Entry("wrong value type for BS", TypedValue{Value: 42, Type: "BS"}),
		)

		It("handles nested map and list conversion", func() {
			tv := TypedValue{
				Type: "M",
				Value: map[string]TypedValue{
					"name": {Value: "root", Type: "S"},
					"items": {
						Type: "L",
						Value: []TypedValue{
							{Value: "item1", Type: "S"},
							{
								Type: "M",
								Value: map[string]TypedValue{
									"nested_key": {Value: "nested_val", Type: "S"},
									"count":      {Value: "5", Type: "N"},
								},
							},
						},
					},
				},
			}

			av, err := convertTypedToAV(tv)
			Expect(err).NotTo(HaveOccurred())

			Expect(av.M).NotTo(BeNil())
			Expect(*av.M["name"].S).To(Equal("root"))

			items := av.M["items"]
			Expect(items.L).To(HaveLen(2))
			Expect(*items.L[0].S).To(Equal("item1"))

			nestedMap := items.L[1]
			Expect(nestedMap.M).NotTo(BeNil())
			Expect(*nestedMap.M["nested_key"].S).To(Equal("nested_val"))
			Expect(*nestedMap.M["count"].N).To(Equal("5"))
		})
	})
})
