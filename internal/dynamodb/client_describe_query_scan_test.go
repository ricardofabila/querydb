package dynamodb

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

// MockDynamoDBAPIAdvanced mocks DescribeTable, Query, and Scan (advanced) API calls
type MockDynamoDBAPIAdvanced struct {
	mock.Mock
}

func (m *MockDynamoDBAPIAdvanced) DescribeTable(input *dynamodb.DescribeTableInput) (*dynamodb.DescribeTableOutput, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.DescribeTableOutput), args.Error(1)
}

func (m *MockDynamoDBAPIAdvanced) Query(input *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.QueryOutput), args.Error(1)
}

func (m *MockDynamoDBAPIAdvanced) Scan(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.ScanOutput), args.Error(1)
}

// TestDynamoDBClientAdvanced wraps the real client with the advanced mock
type TestDynamoDBClientAdvanced struct {
	*DynamoDBClient
	mockAPI *MockDynamoDBAPIAdvanced
}

func newTestClientAdvanced() *TestDynamoDBClientAdvanced {
	mockAPI := &MockDynamoDBAPIAdvanced{}
	client := &DynamoDBClient{
		client: &dynamodb.DynamoDB{},
	}
	return &TestDynamoDBClientAdvanced{
		DynamoDBClient: client,
		mockAPI:        mockAPI,
	}
}

// Override DescribeTable to use mock
func (tc *TestDynamoDBClientAdvanced) DescribeTable(tableName string) (*TableMetadata, error) {
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}
	result, err := tc.mockAPI.DescribeTable(input)
	if err != nil {
		return nil, tc.handleDynamoDBError(err, tableName)
	}
	meta := &TableMetadata{TableName: tableName}
	for _, ks := range result.Table.KeySchema {
		meta.KeySchema = append(meta.KeySchema, KeySchemaElement{
			AttributeName: *ks.AttributeName,
			KeyType:       *ks.KeyType,
		})
	}
	for _, ad := range result.Table.AttributeDefinitions {
		meta.AttributeDefs = append(meta.AttributeDefs, AttributeDef{
			AttributeName: *ad.AttributeName,
			AttributeType: *ad.AttributeType,
		})
	}
	for _, gsi := range result.Table.GlobalSecondaryIndexes {
		idx := IndexInfo{IndexName: *gsi.IndexName, Projection: *gsi.Projection.ProjectionType}
		for _, ks := range gsi.KeySchema {
			idx.KeySchema = append(idx.KeySchema, KeySchemaElement{
				AttributeName: *ks.AttributeName, KeyType: *ks.KeyType,
			})
		}
		meta.GSIs = append(meta.GSIs, idx)
	}
	for _, lsi := range result.Table.LocalSecondaryIndexes {
		idx := IndexInfo{IndexName: *lsi.IndexName, Projection: *lsi.Projection.ProjectionType}
		for _, ks := range lsi.KeySchema {
			idx.KeySchema = append(idx.KeySchema, KeySchemaElement{
				AttributeName: *ks.AttributeName, KeyType: *ks.KeyType,
			})
		}
		meta.LSIs = append(meta.LSIs, idx)
	}
	return meta, nil
}

// Override QueryTable to use mock
func (tc *TestDynamoDBClientAdvanced) QueryTable(params QueryParams) (*QueryResult, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(params.TableName),
		KeyConditionExpression: aws.String(params.KeyConditionExpr),
		ReturnConsumedCapacity: aws.String("TOTAL"),
	}
	if params.IndexName != "" {
		input.IndexName = aws.String(params.IndexName)
	}
	if params.FilterExpr != "" {
		input.FilterExpression = aws.String(params.FilterExpr)
	}
	if params.ProjectionExpr != "" {
		input.ProjectionExpression = aws.String(params.ProjectionExpr)
	}
	if len(params.ExprAttrNames) > 0 {
		names := make(map[string]*string)
		for k, v := range params.ExprAttrNames {
			names[k] = aws.String(v)
		}
		input.ExpressionAttributeNames = names
	}
	if len(params.ExprAttrValues) > 0 {
		input.ExpressionAttributeValues = params.ExprAttrValues
	}
	if params.ExclusiveStartKey != nil {
		input.ExclusiveStartKey = params.ExclusiveStartKey
	}
	if params.Limit != nil {
		input.Limit = params.Limit
	}
	input.ScanIndexForward = aws.Bool(params.ScanForward)

	result, err := tc.mockAPI.Query(input)
	if err != nil {
		return nil, tc.handleDynamoDBError(err, params.TableName)
	}
	return &QueryResult{
		Items:            result.Items,
		Count:            *result.Count,
		ScannedCount:     *result.ScannedCount,
		LastEvaluatedKey: result.LastEvaluatedKey,
		ConsumedCapacity: result.ConsumedCapacity,
	}, nil
}

// Override ScanTableAdvanced to use mock
func (tc *TestDynamoDBClientAdvanced) ScanTableAdvanced(params ScanParams) (*QueryResult, error) {
	input := &dynamodb.ScanInput{
		TableName:              aws.String(params.TableName),
		ReturnConsumedCapacity: aws.String("TOTAL"),
	}
	if params.IndexName != "" {
		input.IndexName = aws.String(params.IndexName)
	}
	if params.FilterExpr != "" {
		input.FilterExpression = aws.String(params.FilterExpr)
	}
	if params.ProjectionExpr != "" {
		input.ProjectionExpression = aws.String(params.ProjectionExpr)
	}
	if len(params.ExprAttrNames) > 0 {
		names := make(map[string]*string)
		for k, v := range params.ExprAttrNames {
			names[k] = aws.String(v)
		}
		input.ExpressionAttributeNames = names
	}
	if len(params.ExprAttrValues) > 0 {
		input.ExpressionAttributeValues = params.ExprAttrValues
	}
	if params.ExclusiveStartKey != nil {
		input.ExclusiveStartKey = params.ExclusiveStartKey
	}
	if params.Limit != nil {
		input.Limit = params.Limit
	}

	result, err := tc.mockAPI.Scan(input)
	if err != nil {
		return nil, tc.handleDynamoDBError(err, params.TableName)
	}
	return &QueryResult{
		Items:            result.Items,
		Count:            *result.Count,
		ScannedCount:     *result.ScannedCount,
		LastEvaluatedKey: result.LastEvaluatedKey,
		ConsumedCapacity: result.ConsumedCapacity,
	}, nil
}

// =====================
// DescribeTable Tests
// =====================

var _ = Describe("DescribeTable", func() {
	It("should return full table metadata on success", func() {
		tc := newTestClientAdvanced()

		mockResponse := &dynamodb.DescribeTableOutput{
			Table: &dynamodb.TableDescription{
				TableName: aws.String("users"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{AttributeName: aws.String("pk"), KeyType: aws.String("HASH")},
					{AttributeName: aws.String("sk"), KeyType: aws.String("RANGE")},
				},
				AttributeDefinitions: []*dynamodb.AttributeDefinition{
					{AttributeName: aws.String("pk"), AttributeType: aws.String("S")},
					{AttributeName: aws.String("sk"), AttributeType: aws.String("S")},
					{AttributeName: aws.String("gsi1pk"), AttributeType: aws.String("S")},
					{AttributeName: aws.String("lsi1sk"), AttributeType: aws.String("N")},
				},
				GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndexDescription{
					{
						IndexName: aws.String("gsi1"),
						KeySchema: []*dynamodb.KeySchemaElement{
							{AttributeName: aws.String("gsi1pk"), KeyType: aws.String("HASH")},
							{AttributeName: aws.String("sk"), KeyType: aws.String("RANGE")},
						},
						Projection: &dynamodb.Projection{ProjectionType: aws.String("ALL")},
					},
				},
				LocalSecondaryIndexes: []*dynamodb.LocalSecondaryIndexDescription{
					{
						IndexName: aws.String("lsi1"),
						KeySchema: []*dynamodb.KeySchemaElement{
							{AttributeName: aws.String("pk"), KeyType: aws.String("HASH")},
							{AttributeName: aws.String("lsi1sk"), KeyType: aws.String("RANGE")},
						},
						Projection: &dynamodb.Projection{ProjectionType: aws.String("KEYS_ONLY")},
					},
				},
			},
		}

		tc.mockAPI.On("DescribeTable", mock.AnythingOfType("*dynamodb.DescribeTableInput")).
			Return(mockResponse, nil)

		meta, err := tc.DescribeTable("users")

		Expect(err).NotTo(HaveOccurred())
		Expect(meta).NotTo(BeNil())
		Expect(meta.TableName).To(Equal("users"))

		// Key schema
		Expect(meta.KeySchema).To(HaveLen(2))
		Expect(meta.KeySchema[0].AttributeName).To(Equal("pk"))
		Expect(meta.KeySchema[0].KeyType).To(Equal("HASH"))
		Expect(meta.KeySchema[1].AttributeName).To(Equal("sk"))
		Expect(meta.KeySchema[1].KeyType).To(Equal("RANGE"))

		// Attribute definitions
		Expect(meta.AttributeDefs).To(HaveLen(4))

		// GSIs
		Expect(meta.GSIs).To(HaveLen(1))
		Expect(meta.GSIs[0].IndexName).To(Equal("gsi1"))
		Expect(meta.GSIs[0].Projection).To(Equal("ALL"))
		Expect(meta.GSIs[0].KeySchema).To(HaveLen(2))
		Expect(meta.GSIs[0].KeySchema[0].AttributeName).To(Equal("gsi1pk"))
		Expect(meta.GSIs[0].KeySchema[0].KeyType).To(Equal("HASH"))

		// LSIs
		Expect(meta.LSIs).To(HaveLen(1))
		Expect(meta.LSIs[0].IndexName).To(Equal("lsi1"))
		Expect(meta.LSIs[0].Projection).To(Equal("KEYS_ONLY"))
		Expect(meta.LSIs[0].KeySchema).To(HaveLen(2))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should handle a table with no indexes", func() {
		tc := newTestClientAdvanced()

		mockResponse := &dynamodb.DescribeTableOutput{
			Table: &dynamodb.TableDescription{
				TableName: aws.String("simple-table"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{AttributeName: aws.String("id"), KeyType: aws.String("HASH")},
				},
				AttributeDefinitions: []*dynamodb.AttributeDefinition{
					{AttributeName: aws.String("id"), AttributeType: aws.String("S")},
				},
				GlobalSecondaryIndexes: nil,
				LocalSecondaryIndexes:  nil,
			},
		}

		tc.mockAPI.On("DescribeTable", mock.AnythingOfType("*dynamodb.DescribeTableInput")).
			Return(mockResponse, nil)

		meta, err := tc.DescribeTable("simple-table")

		Expect(err).NotTo(HaveOccurred())
		Expect(meta).NotTo(BeNil())
		Expect(meta.TableName).To(Equal("simple-table"))
		Expect(meta.KeySchema).To(HaveLen(1))
		Expect(meta.KeySchema[0].AttributeName).To(Equal("id"))
		Expect(meta.KeySchema[0].KeyType).To(Equal("HASH"))
		Expect(meta.AttributeDefs).To(HaveLen(1))
		Expect(meta.GSIs).To(BeNil())
		Expect(meta.LSIs).To(BeNil())

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should return an error when DescribeTable fails", func() {
		tc := newTestClientAdvanced()

		tc.mockAPI.On("DescribeTable", mock.AnythingOfType("*dynamodb.DescribeTableInput")).
			Return((*dynamodb.DescribeTableOutput)(nil), errors.New("table not found"))

		meta, err := tc.DescribeTable("non-existent-table")

		Expect(err).To(HaveOccurred())
		Expect(meta).To(BeNil())
		// handleDynamoDBError wraps non-AWS errors
		Expect(err.Error()).To(ContainSubstring("Unexpected error during table scan"))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})
})

// =====================
// QueryTable Tests
// =====================

var _ = Describe("QueryTable", func() {
	It("should return results on success", func() {
		tc := newTestClientAdvanced()

		mockResponse := &dynamodb.QueryOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"pk":   {S: aws.String("user-1")},
					"sk":   {S: aws.String("PROFILE")},
					"name": {S: aws.String("Alice")},
				},
				{
					"pk":    {S: aws.String("user-1")},
					"sk":    {S: aws.String("ORDER#001")},
					"total": {N: aws.String("99.99")},
				},
			},
			Count:            aws.Int64(2),
			ScannedCount:     aws.Int64(2),
			LastEvaluatedKey: nil,
			ConsumedCapacity: &dynamodb.ConsumedCapacity{
				TableName:     aws.String("users"),
				CapacityUnits: aws.Float64(5.0),
			},
		}

		tc.mockAPI.On("Query", mock.AnythingOfType("*dynamodb.QueryInput")).
			Return(mockResponse, nil)

		params := QueryParams{
			TableName:        "users",
			KeyConditionExpr: "#pk = :pkval",
			ExprAttrNames:    map[string]string{"#pk": "pk"},
			ExprAttrValues: map[string]*dynamodb.AttributeValue{
				":pkval": {S: aws.String("user-1")},
			},
			ScanForward: true,
		}

		result, err := tc.QueryTable(params)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Items).To(HaveLen(2))
		Expect(result.Count).To(Equal(int64(2)))
		Expect(result.ScannedCount).To(Equal(int64(2)))
		Expect(result.LastEvaluatedKey).To(BeNil())
		Expect(result.ConsumedCapacity).NotTo(BeNil())
		Expect(*result.ConsumedCapacity.CapacityUnits).To(Equal(float64(5.0)))

		// Verify first item
		Expect(*result.Items[0]["pk"].S).To(Equal("user-1"))
		Expect(*result.Items[0]["sk"].S).To(Equal("PROFILE"))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should query with an index", func() {
		tc := newTestClientAdvanced()

		mockResponse := &dynamodb.QueryOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"gsi1pk": {S: aws.String("ORG-1")},
					"sk":     {S: aws.String("user-1")},
					"name":   {S: aws.String("Alice")},
				},
			},
			Count:            aws.Int64(1),
			ScannedCount:     aws.Int64(1),
			LastEvaluatedKey: nil,
			ConsumedCapacity: &dynamodb.ConsumedCapacity{
				TableName:     aws.String("users"),
				CapacityUnits: aws.Float64(2.0),
			},
		}

		tc.mockAPI.On("Query", mock.MatchedBy(func(input *dynamodb.QueryInput) bool {
			return input.IndexName != nil && *input.IndexName == "gsi1"
		})).Return(mockResponse, nil)

		params := QueryParams{
			TableName:        "users",
			IndexName:        "gsi1",
			KeyConditionExpr: "#pk = :pkval",
			ExprAttrNames:    map[string]string{"#pk": "gsi1pk"},
			ExprAttrValues: map[string]*dynamodb.AttributeValue{
				":pkval": {S: aws.String("ORG-1")},
			},
			ScanForward: true,
		}

		result, err := tc.QueryTable(params)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Items).To(HaveLen(1))
		Expect(*result.Items[0]["gsi1pk"].S).To(Equal("ORG-1"))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should handle pagination with ExclusiveStartKey", func() {
		tc := newTestClientAdvanced()

		lastKey := map[string]*dynamodb.AttributeValue{
			"pk": {S: aws.String("user-1")},
			"sk": {S: aws.String("ORDER#050")},
		}

		mockResponse := &dynamodb.QueryOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"pk": {S: aws.String("user-1")},
					"sk": {S: aws.String("ORDER#051")},
				},
			},
			Count:            aws.Int64(1),
			ScannedCount:     aws.Int64(1),
			LastEvaluatedKey: map[string]*dynamodb.AttributeValue{
				"pk": {S: aws.String("user-1")},
				"sk": {S: aws.String("ORDER#051")},
			},
			ConsumedCapacity: &dynamodb.ConsumedCapacity{
				TableName:     aws.String("users"),
				CapacityUnits: aws.Float64(1.0),
			},
		}

		tc.mockAPI.On("Query", mock.MatchedBy(func(input *dynamodb.QueryInput) bool {
			return input.ExclusiveStartKey != nil
		})).Return(mockResponse, nil)

		params := QueryParams{
			TableName:        "users",
			KeyConditionExpr: "#pk = :pkval",
			ExprAttrNames:    map[string]string{"#pk": "pk"},
			ExprAttrValues: map[string]*dynamodb.AttributeValue{
				":pkval": {S: aws.String("user-1")},
			},
			ExclusiveStartKey: lastKey,
			ScanForward:       true,
		}

		result, err := tc.QueryTable(params)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Items).To(HaveLen(1))
		// Verify LastEvaluatedKey is returned for further pagination
		Expect(result.LastEvaluatedKey).NotTo(BeNil())
		Expect(*result.LastEvaluatedKey["sk"].S).To(Equal("ORDER#051"))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should handle empty results", func() {
		tc := newTestClientAdvanced()

		mockResponse := &dynamodb.QueryOutput{
			Items:            []map[string]*dynamodb.AttributeValue{},
			Count:            aws.Int64(0),
			ScannedCount:     aws.Int64(0),
			LastEvaluatedKey: nil,
			ConsumedCapacity: &dynamodb.ConsumedCapacity{
				TableName:     aws.String("users"),
				CapacityUnits: aws.Float64(0.5),
			},
		}

		tc.mockAPI.On("Query", mock.AnythingOfType("*dynamodb.QueryInput")).
			Return(mockResponse, nil)

		params := QueryParams{
			TableName:        "users",
			KeyConditionExpr: "#pk = :pkval",
			ExprAttrNames:    map[string]string{"#pk": "pk"},
			ExprAttrValues: map[string]*dynamodb.AttributeValue{
				":pkval": {S: aws.String("nonexistent-user")},
			},
			ScanForward: true,
		}

		result, err := tc.QueryTable(params)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Items).To(BeEmpty())
		Expect(result.Count).To(Equal(int64(0)))
		Expect(result.ScannedCount).To(Equal(int64(0)))
		Expect(result.LastEvaluatedKey).To(BeNil())

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should return an error on query failure", func() {
		tc := newTestClientAdvanced()

		tc.mockAPI.On("Query", mock.AnythingOfType("*dynamodb.QueryInput")).
			Return((*dynamodb.QueryOutput)(nil), errors.New("validation error: key condition not valid"))

		params := QueryParams{
			TableName:        "users",
			KeyConditionExpr: "bad expression",
		}

		result, err := tc.QueryTable(params)

		Expect(err).To(HaveOccurred())
		Expect(result).To(BeNil())
		// handleDynamoDBError wraps non-AWS errors
		Expect(err.Error()).To(ContainSubstring("Unexpected error during table scan"))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})
})

// ==============================
// ScanTableAdvanced Tests
// ==============================

var _ = Describe("ScanTableAdvanced", func() {
	It("should return results on success", func() {
		tc := newTestClientAdvanced()

		mockResponse := &dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id":   {S: aws.String("item-1")},
					"name": {S: aws.String("First")},
				},
				{
					"id":   {S: aws.String("item-2")},
					"name": {S: aws.String("Second")},
				},
			},
			Count:            aws.Int64(2),
			ScannedCount:     aws.Int64(10),
			LastEvaluatedKey: nil,
			ConsumedCapacity: &dynamodb.ConsumedCapacity{
				TableName:     aws.String("items"),
				CapacityUnits: aws.Float64(8.0),
			},
		}

		tc.mockAPI.On("Scan", mock.AnythingOfType("*dynamodb.ScanInput")).
			Return(mockResponse, nil)

		params := ScanParams{
			TableName: "items",
		}

		result, err := tc.ScanTableAdvanced(params)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Items).To(HaveLen(2))
		Expect(result.Count).To(Equal(int64(2)))
		Expect(result.ScannedCount).To(Equal(int64(10)))
		Expect(result.LastEvaluatedKey).To(BeNil())
		Expect(result.ConsumedCapacity).NotTo(BeNil())
		Expect(*result.ConsumedCapacity.CapacityUnits).To(Equal(float64(8.0)))

		// Verify items
		Expect(*result.Items[0]["id"].S).To(Equal("item-1"))
		Expect(*result.Items[1]["id"].S).To(Equal("item-2"))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should scan with a filter expression", func() {
		tc := newTestClientAdvanced()

		mockResponse := &dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id":  {S: aws.String("item-1")},
					"age": {N: aws.String("30")},
				},
			},
			Count:            aws.Int64(1),
			ScannedCount:     aws.Int64(50),
			LastEvaluatedKey: nil,
			ConsumedCapacity: &dynamodb.ConsumedCapacity{
				TableName:     aws.String("items"),
				CapacityUnits: aws.Float64(12.5),
			},
		}

		tc.mockAPI.On("Scan", mock.MatchedBy(func(input *dynamodb.ScanInput) bool {
			return input.FilterExpression != nil && *input.FilterExpression == "#age > :minAge"
		})).Return(mockResponse, nil)

		params := ScanParams{
			TableName:  "items",
			FilterExpr: "#age > :minAge",
			ExprAttrNames: map[string]string{
				"#age": "age",
			},
			ExprAttrValues: map[string]*dynamodb.AttributeValue{
				":minAge": {N: aws.String("25")},
			},
		}

		result, err := tc.ScanTableAdvanced(params)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Items).To(HaveLen(1))
		Expect(result.Count).To(Equal(int64(1)))
		Expect(result.ScannedCount).To(Equal(int64(50)))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should handle pagination with ExclusiveStartKey", func() {
		tc := newTestClientAdvanced()

		startKey := map[string]*dynamodb.AttributeValue{
			"id": {S: aws.String("item-50")},
		}

		mockResponse := &dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id":   {S: aws.String("item-51")},
					"name": {S: aws.String("Next page item")},
				},
			},
			Count:        aws.Int64(1),
			ScannedCount: aws.Int64(1),
			LastEvaluatedKey: map[string]*dynamodb.AttributeValue{
				"id": {S: aws.String("item-51")},
			},
			ConsumedCapacity: &dynamodb.ConsumedCapacity{
				TableName:     aws.String("items"),
				CapacityUnits: aws.Float64(1.0),
			},
		}

		tc.mockAPI.On("Scan", mock.MatchedBy(func(input *dynamodb.ScanInput) bool {
			return input.ExclusiveStartKey != nil
		})).Return(mockResponse, nil)

		limit := int64(1)
		params := ScanParams{
			TableName:         "items",
			ExclusiveStartKey: startKey,
			Limit:             &limit,
		}

		result, err := tc.ScanTableAdvanced(params)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Items).To(HaveLen(1))
		Expect(result.LastEvaluatedKey).NotTo(BeNil())
		Expect(*result.LastEvaluatedKey["id"].S).To(Equal("item-51"))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should return an error on scan failure", func() {
		tc := newTestClientAdvanced()

		tc.mockAPI.On("Scan", mock.AnythingOfType("*dynamodb.ScanInput")).
			Return((*dynamodb.ScanOutput)(nil), errors.New("access denied"))

		params := ScanParams{
			TableName: "restricted-table",
		}

		result, err := tc.ScanTableAdvanced(params)

		Expect(err).To(HaveOccurred())
		Expect(result).To(BeNil())
		// handleDynamoDBError wraps non-AWS errors
		Expect(err.Error()).To(ContainSubstring("Unexpected error during table scan"))

		tc.mockAPI.AssertExpectations(GinkgoT())
	})
})
