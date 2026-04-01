package dynamodb

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	qerrors "querydb/internal/errors"
)

func TestDynamoDB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DynamoDB Suite")
}

// MockDynamoDBAPI is a mock implementation of the DynamoDB API
type MockDynamoDBAPI struct {
	mock.Mock
}

func (m *MockDynamoDBAPI) Scan(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*dynamodb.ScanOutput), args.Error(1)
}

// TestDynamoDBClient wraps the real client with a mock
type TestDynamoDBClient struct {
	*DynamoDBClient
	mockAPI *MockDynamoDBAPI
}

func newTestClient() *TestDynamoDBClient {
	mockAPI := &MockDynamoDBAPI{}
	client := &DynamoDBClient{
		client: &dynamodb.DynamoDB{},
	}

	testClient := &TestDynamoDBClient{
		DynamoDBClient: client,
		mockAPI:        mockAPI,
	}

	return testClient
}

// Override ScanTable to use mock
func (tc *TestDynamoDBClient) ScanTable(tableName string) ([]map[string]interface{}, error) {
	var items []map[string]interface{}
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName: aws.String(tableName),
		}

		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := tc.mockAPI.Scan(input)
		if err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			convertedItem, err := convertDynamoDBItem(item)
			if err != nil {
				return nil, err
			}
			items = append(items, convertedItem)
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return items, nil
}

// MockDynamoDBAPIWithPut extends MockDynamoDBAPI with PutItem support
type MockDynamoDBAPIWithPut struct {
	MockDynamoDBAPI
}

func (m *MockDynamoDBAPIWithPut) PutItem(input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}

func (m *MockDynamoDBAPIWithPut) UpdateItem(input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.UpdateItemOutput), args.Error(1)
}

func (m *MockDynamoDBAPIWithPut) DeleteItem(input *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.DeleteItemOutput), args.Error(1)
}

// TestDynamoDBClientWithPut wraps the client with PutItem mock support
type TestDynamoDBClientWithPut struct {
	*DynamoDBClient
	mockAPI *MockDynamoDBAPIWithPut
}

func newTestClientWithPut() *TestDynamoDBClientWithPut {
	mockAPI := &MockDynamoDBAPIWithPut{}
	client := &DynamoDBClient{
		client: &dynamodb.DynamoDB{},
	}
	testClient := &TestDynamoDBClientWithPut{
		DynamoDBClient: client,
		mockAPI:        mockAPI,
	}
	return testClient
}

// Override PutItem to use mock
func (tc *TestDynamoDBClientWithPut) PutItem(tableName string, item map[string]interface{}) error {
	attributeMap, err := convertToAttributeValueMap(item)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      attributeMap,
	}

	_, err = tc.mockAPI.PutItem(input)
	if err != nil {
		return tc.handleDynamoDBError(err, tableName)
	}

	return nil
}

// Override UpdateItem to use mock
func (tc *TestDynamoDBClientWithPut) UpdateItem(tableName string, key map[string]interface{}, updates map[string]interface{}) error {
	keyAttributeMap, err := convertToAttributeValueMap(key)
	if err != nil {
		return qerrors.NewValidationError(
			"Failed to convert key to DynamoDB format",
			err,
			"Check that all key attributes are valid DynamoDB types",
			"Ensure key structure matches the table's key schema",
		)
	}

	updateExpression, expressionAttributeNames, expressionAttributeValues, err := buildUpdateExpression(updates)
	if err != nil {
		return qerrors.NewValidationError(
			"Failed to build update expression",
			err,
			"Check that all update values are valid DynamoDB types",
			"Ensure attribute names don't conflict with reserved words",
		)
	}

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(tableName),
		Key:                       keyAttributeMap,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
	}

	_, err = tc.mockAPI.UpdateItem(input)
	if err != nil {
		return tc.handleDynamoDBError(err, tableName)
	}

	return nil
}

// Override DeleteItem to use mock
func (tc *TestDynamoDBClientWithPut) DeleteItem(tableName string, key map[string]interface{}) error {
	keyAttributeMap, err := convertToAttributeValueMap(key)
	if err != nil {
		return qerrors.NewValidationError(
			"Failed to convert key to DynamoDB format",
			err,
			"Check that all key attributes are valid DynamoDB types",
			"Ensure key structure matches the table's key schema",
		)
	}

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       keyAttributeMap,
	}

	_, err = tc.mockAPI.DeleteItem(input)
	if err != nil {
		return tc.handleDynamoDBError(err, tableName)
	}

	return nil
}

// MockDynamoDBAPIWithGetItem extends MockDynamoDBAPIWithPut with GetItem support
type MockDynamoDBAPIWithGetItem struct {
	MockDynamoDBAPIWithPut
}

func (m *MockDynamoDBAPIWithGetItem) GetItem(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.GetItemOutput), args.Error(1)
}

// TestDynamoDBClientWithGetItem wraps the client with GetItem mock support
type TestDynamoDBClientWithGetItem struct {
	*DynamoDBClient
	mockAPI *MockDynamoDBAPIWithGetItem
}

func newTestClientWithGetItem() *TestDynamoDBClientWithGetItem {
	mockAPI := &MockDynamoDBAPIWithGetItem{}
	client := &DynamoDBClient{
		client: &dynamodb.DynamoDB{},
	}
	testClient := &TestDynamoDBClientWithGetItem{
		DynamoDBClient: client,
		mockAPI:        mockAPI,
	}
	return testClient
}

// Override GetItem to use mock
func (tc *TestDynamoDBClientWithGetItem) GetItem(tableName string, key map[string]interface{}) (map[string]interface{}, error) {
	keyAttributeMap, err := convertToAttributeValueMap(key)
	if err != nil {
		return nil, qerrors.NewValidationError(
			"Failed to convert key to DynamoDB format",
			err,
			"Check that all key attributes are valid DynamoDB types",
			"Ensure key structure matches the table's key schema",
		)
	}

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       keyAttributeMap,
	}

	result, err := tc.mockAPI.GetItem(input)
	if err != nil {
		return nil, tc.handleDynamoDBError(err, tableName)
	}

	if result.Item == nil {
		return nil, qerrors.NewQueryError(
			fmt.Sprintf("Item not found in table '%s'", tableName),
			nil,
			"Verify the key values are correct",
			"Check that the item exists in the table",
			"Ensure the key structure matches the table's key schema",
		)
	}

	item, err := convertDynamoDBItem(result.Item)
	if err != nil {
		return nil, qerrors.NewFormatError(
			"Failed to convert DynamoDB item to readable format",
			err,
			"This might be due to unsupported data types",
			"Check if the item contains complex nested structures",
			"Try querying a different item to isolate the issue",
		)
	}

	return item, nil
}

// Override PutItem to use mock
func (tc *TestDynamoDBClientWithGetItem) PutItem(tableName string, item map[string]interface{}) error {
	attributeMap, err := convertToAttributeValueMap(item)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      attributeMap,
	}

	_, err = tc.mockAPI.PutItem(input)
	if err != nil {
		return tc.handleDynamoDBError(err, tableName)
	}

	return nil
}

// Override UpdateItem to use mock
func (tc *TestDynamoDBClientWithGetItem) UpdateItem(tableName string, key map[string]interface{}, updates map[string]interface{}) error {
	keyAttributeMap, err := convertToAttributeValueMap(key)
	if err != nil {
		return qerrors.NewValidationError(
			"Failed to convert key to DynamoDB format",
			err,
			"Check that all key attributes are valid DynamoDB types",
			"Ensure key structure matches the table's key schema",
		)
	}

	updateExpression, expressionAttributeNames, expressionAttributeValues, err := buildUpdateExpression(updates)
	if err != nil {
		return qerrors.NewValidationError(
			"Failed to build update expression",
			err,
			"Check that all update values are valid DynamoDB types",
			"Ensure attribute names don't conflict with reserved words",
		)
	}

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(tableName),
		Key:                       keyAttributeMap,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
	}

	_, err = tc.mockAPI.UpdateItem(input)
	if err != nil {
		return tc.handleDynamoDBError(err, tableName)
	}

	return nil
}

// Override DeleteItem to use mock
func (tc *TestDynamoDBClientWithGetItem) DeleteItem(tableName string, key map[string]interface{}) error {
	keyAttributeMap, err := convertToAttributeValueMap(key)
	if err != nil {
		return qerrors.NewValidationError(
			"Failed to convert key to DynamoDB format",
			err,
			"Check that all key attributes are valid DynamoDB types",
			"Ensure key structure matches the table's key schema",
		)
	}

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       keyAttributeMap,
	}

	_, err = tc.mockAPI.DeleteItem(input)
	if err != nil {
		return tc.handleDynamoDBError(err, tableName)
	}

	return nil
}

var _ = Describe("NewClient", func() {
	DescribeTable("creating a client",
		func(endpoint, region, accessKey, secretKey string, wantErr bool) {
			client, err := NewClient(endpoint, region, accessKey, secretKey)
			if wantErr {
				Expect(err).To(HaveOccurred())
				Expect(client).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())
				err = client.Close()
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("valid configuration", "http://localhost:8000", "us-east-1", "test-key", "test-secret", false),
		Entry("valid configuration without credentials", "http://localhost:8000", "us-east-1", "", "", false),
		Entry("valid configuration without endpoint", "", "us-east-1", "test-key", "test-secret", false),
	)
})

var _ = Describe("ScanTable", func() {
	It("should scan a single page of results", func() {
		testClient := newTestClient()

		mockResponse := &dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id":   {S: aws.String("123")},
					"name": {S: aws.String("test-item")},
					"age":  {N: aws.String("25")},
				},
				{
					"id":   {S: aws.String("456")},
					"name": {S: aws.String("another-item")},
					"age":  {N: aws.String("30")},
				},
			},
			LastEvaluatedKey: nil,
		}

		testClient.mockAPI.On("Scan", mock.AnythingOfType("*dynamodb.ScanInput")).Return(mockResponse, nil)

		items, err := testClient.ScanTable("test-table")

		Expect(err).NotTo(HaveOccurred())
		Expect(items).To(HaveLen(2))

		Expect(items[0]["id"]).To(Equal("123"))
		Expect(items[0]["name"]).To(Equal("test-item"))
		Expect(items[0]["age"]).To(Equal(int64(25)))

		Expect(items[1]["id"]).To(Equal("456"))
		Expect(items[1]["name"]).To(Equal("another-item"))
		Expect(items[1]["age"]).To(Equal(int64(30)))

		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should scan multiple pages of results", func() {
		testClient := newTestClient()

		firstPageResponse := &dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id":   {S: aws.String("123")},
					"name": {S: aws.String("first-page-item")},
				},
			},
			LastEvaluatedKey: map[string]*dynamodb.AttributeValue{
				"id": {S: aws.String("123")},
			},
		}

		secondPageResponse := &dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id":   {S: aws.String("456")},
					"name": {S: aws.String("second-page-item")},
				},
			},
			LastEvaluatedKey: nil,
		}

		testClient.mockAPI.On("Scan", mock.MatchedBy(func(input *dynamodb.ScanInput) bool {
			return input.ExclusiveStartKey == nil
		})).Return(firstPageResponse, nil).Once()

		testClient.mockAPI.On("Scan", mock.MatchedBy(func(input *dynamodb.ScanInput) bool {
			return input.ExclusiveStartKey != nil
		})).Return(secondPageResponse, nil).Once()

		items, err := testClient.ScanTable("test-table")

		Expect(err).NotTo(HaveOccurred())
		Expect(items).To(HaveLen(2))

		Expect(items[0]["id"]).To(Equal("123"))
		Expect(items[0]["name"]).To(Equal("first-page-item"))
		Expect(items[1]["id"]).To(Equal("456"))
		Expect(items[1]["name"]).To(Equal("second-page-item"))

		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should return an error on scan failure", func() {
		testClient := newTestClient()

		testClient.mockAPI.On("Scan", mock.AnythingOfType("*dynamodb.ScanInput")).Return(
			(*dynamodb.ScanOutput)(nil),
			errors.New("table not found"),
		)

		items, err := testClient.ScanTable("non-existent-table")

		Expect(err).To(HaveOccurred())
		Expect(items).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("table not found"))

		testClient.mockAPI.AssertExpectations(GinkgoT())
	})
})

var _ = Describe("convertAttributeValue", func() {
	It("should convert a string", func() {
		av := &dynamodb.AttributeValue{S: aws.String("test-string")}
		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("test-string"))
	})

	DescribeTable("should convert numbers",
		func(input string, expected interface{}) {
			av := &dynamodb.AttributeValue{N: aws.String(input)}
			result, err := convertAttributeValue(av)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
		Entry("integer", "123", int64(123)),
		Entry("negative integer", "-456", int64(-456)),
		Entry("float", "123.45", float64(123.45)),
		Entry("negative float", "-67.89", float64(-67.89)),
		Entry("scientific notation", "1.23e10", float64(1.23e10)),
	)

	DescribeTable("should convert booleans",
		func(input bool, expected bool) {
			av := &dynamodb.AttributeValue{BOOL: aws.Bool(input)}
			result, err := convertAttributeValue(av)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
		Entry("true", true, true),
		Entry("false", false, false),
	)

	It("should convert null", func() {
		av := &dynamodb.AttributeValue{NULL: aws.Bool(true)}
		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("should convert a list", func() {
		av := &dynamodb.AttributeValue{
			L: []*dynamodb.AttributeValue{
				{S: aws.String("item1")},
				{N: aws.String("42")},
				{BOOL: aws.Bool(true)},
				{NULL: aws.Bool(true)},
			},
		}

		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		list, ok := result.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(list).To(HaveLen(4))
		Expect(list[0]).To(Equal("item1"))
		Expect(list[1]).To(Equal(int64(42)))
		Expect(list[2]).To(Equal(true))
		Expect(list[3]).To(BeNil())
	})

	It("should convert a map", func() {
		av := &dynamodb.AttributeValue{
			M: map[string]*dynamodb.AttributeValue{
				"string_field": {S: aws.String("value")},
				"number_field": {N: aws.String("123")},
				"bool_field":   {BOOL: aws.Bool(false)},
				"null_field":   {NULL: aws.Bool(true)},
			},
		}

		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		resultMap, ok := result.(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(resultMap).To(HaveLen(4))
		Expect(resultMap["string_field"]).To(Equal("value"))
		Expect(resultMap["number_field"]).To(Equal(int64(123)))
		Expect(resultMap["bool_field"]).To(Equal(false))
		Expect(resultMap["null_field"]).To(BeNil())
	})

	It("should convert a string set", func() {
		av := &dynamodb.AttributeValue{
			SS: []*string{
				aws.String("item1"),
				aws.String("item2"),
				aws.String("item3"),
			},
		}

		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		stringSet, ok := result.([]string)
		Expect(ok).To(BeTrue())
		Expect(stringSet).To(HaveLen(3))
		Expect(stringSet).To(Equal([]string{"item1", "item2", "item3"}))
	})

	It("should convert a number set", func() {
		av := &dynamodb.AttributeValue{
			NS: []*string{
				aws.String("123"),
				aws.String("456.78"),
				aws.String("999"),
			},
		}

		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		numberSet, ok := result.([]interface{})
		Expect(ok).To(BeTrue())
		Expect(numberSet).To(HaveLen(3))
		Expect(numberSet[0]).To(Equal(int64(123)))
		Expect(numberSet[1]).To(Equal(float64(456.78)))
		Expect(numberSet[2]).To(Equal(int64(999)))
	})

	It("should convert a binary set", func() {
		av := &dynamodb.AttributeValue{
			BS: [][]byte{
				[]byte("binary1"),
				[]byte("binary2"),
			},
		}

		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		binarySet, ok := result.([]string)
		Expect(ok).To(BeTrue())
		Expect(binarySet).To(HaveLen(2))
		Expect(binarySet).To(Equal([]string{"binary1", "binary2"}))
	})

	It("should convert binary data", func() {
		av := &dynamodb.AttributeValue{
			B: []byte("binary-data"),
		}

		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("binary-data"))
	})

	It("should handle unsupported type", func() {
		av := &dynamodb.AttributeValue{}
		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.(string)).To(ContainSubstring("unsupported_type"))
	})

	It("should handle invalid number", func() {
		av := &dynamodb.AttributeValue{N: aws.String("invalid-number")}
		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("invalid-number"))
	})

	It("should handle nested structures", func() {
		av := &dynamodb.AttributeValue{
			M: map[string]*dynamodb.AttributeValue{
				"nested_list": {
					L: []*dynamodb.AttributeValue{
						{
							M: map[string]*dynamodb.AttributeValue{
								"inner_string": {S: aws.String("nested_value")},
								"inner_number": {N: aws.String("42")},
							},
						},
						{S: aws.String("list_item")},
					},
				},
				"simple_field": {S: aws.String("simple_value")},
			},
		}

		result, err := convertAttributeValue(av)
		Expect(err).NotTo(HaveOccurred())
		resultMap, ok := result.(map[string]interface{})
		Expect(ok).To(BeTrue())

		nestedList, ok := resultMap["nested_list"].([]interface{})
		Expect(ok).To(BeTrue())
		Expect(nestedList).To(HaveLen(2))

		nestedMap, ok := nestedList[0].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(nestedMap["inner_string"]).To(Equal("nested_value"))
		Expect(nestedMap["inner_number"]).To(Equal(int64(42)))

		Expect(nestedList[1]).To(Equal("list_item"))
		Expect(resultMap["simple_field"]).To(Equal("simple_value"))
	})
})

var _ = Describe("convertDynamoDBItem", func() {
	It("should convert a full item", func() {
		item := map[string]*dynamodb.AttributeValue{
			"id":       {S: aws.String("123")},
			"name":     {S: aws.String("test-item")},
			"age":      {N: aws.String("25")},
			"active":   {BOOL: aws.Bool(true)},
			"metadata": {NULL: aws.Bool(true)},
			"tags": {
				L: []*dynamodb.AttributeValue{
					{S: aws.String("tag1")},
					{S: aws.String("tag2")},
				},
			},
			"config": {
				M: map[string]*dynamodb.AttributeValue{
					"setting1": {S: aws.String("value1")},
					"setting2": {N: aws.String("100")},
				},
			},
		}

		result, err := convertDynamoDBItem(item)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(HaveLen(7))
		Expect(result["id"]).To(Equal("123"))
		Expect(result["name"]).To(Equal("test-item"))
		Expect(result["age"]).To(Equal(int64(25)))
		Expect(result["active"]).To(Equal(true))
		Expect(result["metadata"]).To(BeNil())

		tags, ok := result["tags"].([]interface{})
		Expect(ok).To(BeTrue())
		Expect(tags).To(HaveLen(2))
		Expect(tags[0]).To(Equal("tag1"))
		Expect(tags[1]).To(Equal("tag2"))

		config, ok := result["config"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(config).To(HaveLen(2))
		Expect(config["setting1"]).To(Equal("value1"))
		Expect(config["setting2"]).To(Equal(int64(100)))
	})

	It("should handle complex nested structures without error", func() {
		item := map[string]*dynamodb.AttributeValue{
			"valid_field": {S: aws.String("valid")},
			"nested_field": {
				L: []*dynamodb.AttributeValue{
					{S: aws.String("valid_nested")},
					{
						M: map[string]*dynamodb.AttributeValue{
							"problem_field": {
								L: []*dynamodb.AttributeValue{
									{
										M: map[string]*dynamodb.AttributeValue{
											"deep_field": {S: aws.String("deep_value")},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		result, err := convertDynamoDBItem(item)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result["valid_field"]).To(Equal("valid"))
	})
})

var _ = Describe("parseNumber", func() {
	DescribeTable("parsing numbers",
		func(input string, expected interface{}, expectError bool) {
			result, err := parseNumber(input)
			if expectError {
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expected))
			}
		},
		Entry("positive integer", "123", int64(123), false),
		Entry("negative integer", "-456", int64(-456), false),
		Entry("zero", "0", int64(0), false),
		Entry("positive float", "123.45", float64(123.45), false),
		Entry("negative float", "-67.89", float64(-67.89), false),
		Entry("scientific notation", "1.23e10", float64(1.23e10), false),
		Entry("invalid number", "not-a-number", nil, true),
		Entry("empty string", "", nil, true),
	)
})

var _ = Describe("PutItem", func() {
	It("should put an item successfully", func() {
		testClient := newTestClientWithPut()

		item := map[string]interface{}{
			"id":     "test-123",
			"name":   "Test Item",
			"count":  int64(42),
			"active": true,
		}

		testClient.mockAPI.On("PutItem", mock.AnythingOfType("*dynamodb.PutItemInput")).
			Return(&dynamodb.PutItemOutput{}, nil)

		err := testClient.PutItem("test-table", item)
		Expect(err).NotTo(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should put an item with nested structures", func() {
		testClient := newTestClientWithPut()

		item := map[string]interface{}{
			"id":   "test-456",
			"name": "Complex Item",
			"metadata": map[string]interface{}{
				"created": "2024-01-01",
				"version": int64(1),
			},
			"tags": []interface{}{"tag1", "tag2", "tag3"},
		}

		testClient.mockAPI.On("PutItem", mock.AnythingOfType("*dynamodb.PutItemInput")).
			Return(&dynamodb.PutItemOutput{}, nil)

		err := testClient.PutItem("test-table", item)
		Expect(err).NotTo(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should put an item with null value", func() {
		testClient := newTestClientWithPut()

		item := map[string]interface{}{
			"id":          "test-789",
			"name":        "Item with Null",
			"description": nil,
		}

		testClient.mockAPI.On("PutItem", mock.AnythingOfType("*dynamodb.PutItemInput")).
			Return(&dynamodb.PutItemOutput{}, nil)

		err := testClient.PutItem("test-table", item)
		Expect(err).NotTo(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should return error on DynamoDB failure", func() {
		testClient := newTestClientWithPut()

		item := map[string]interface{}{
			"id":   "test-error",
			"name": "Error Item",
		}

		testClient.mockAPI.On("PutItem", mock.AnythingOfType("*dynamodb.PutItemInput")).
			Return((*dynamodb.PutItemOutput)(nil), errors.New("dynamodb error"))

		err := testClient.PutItem("test-table", item)
		Expect(err).To(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})
})

var _ = Describe("convertToAttributeValue", func() {
	It("should convert a string", func() {
		av, err := convertToAttributeValue("test-string")
		Expect(err).NotTo(HaveOccurred())
		Expect(av.S).NotTo(BeNil())
		Expect(*av.S).To(Equal("test-string"))
	})

	It("should convert an int", func() {
		av, err := convertToAttributeValue(123)
		Expect(err).NotTo(HaveOccurred())
		Expect(av.N).NotTo(BeNil())
		Expect(*av.N).To(Equal("123"))
	})

	It("should convert an int64", func() {
		av, err := convertToAttributeValue(int64(456))
		Expect(err).NotTo(HaveOccurred())
		Expect(av.N).NotTo(BeNil())
		Expect(*av.N).To(Equal("456"))
	})

	It("should convert a float64", func() {
		av, err := convertToAttributeValue(123.45)
		Expect(err).NotTo(HaveOccurred())
		Expect(av.N).NotTo(BeNil())
		Expect(*av.N).To(Equal("123.45"))
	})

	DescribeTable("should convert booleans",
		func(input bool, expected bool) {
			av, err := convertToAttributeValue(input)
			Expect(err).NotTo(HaveOccurred())
			Expect(av.BOOL).NotTo(BeNil())
			Expect(*av.BOOL).To(Equal(expected))
		},
		Entry("true", true, true),
		Entry("false", false, false),
	)

	It("should convert null", func() {
		av, err := convertToAttributeValue(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(av.NULL).NotTo(BeNil())
		Expect(*av.NULL).To(BeTrue())
	})

	It("should convert a list", func() {
		list := []interface{}{
			"string-item",
			int64(42),
			true,
			nil,
		}

		av, err := convertToAttributeValue(list)
		Expect(err).NotTo(HaveOccurred())
		Expect(av.L).NotTo(BeNil())
		Expect(av.L).To(HaveLen(4))

		Expect(av.L[0].S).NotTo(BeNil())
		Expect(*av.L[0].S).To(Equal("string-item"))
		Expect(av.L[1].N).NotTo(BeNil())
		Expect(*av.L[1].N).To(Equal("42"))
		Expect(av.L[2].BOOL).NotTo(BeNil())
		Expect(*av.L[2].BOOL).To(BeTrue())
		Expect(av.L[3].NULL).NotTo(BeNil())
		Expect(*av.L[3].NULL).To(BeTrue())
	})

	It("should convert a map", func() {
		m := map[string]interface{}{
			"string_field": "value",
			"number_field": int64(123),
			"bool_field":   false,
			"null_field":   nil,
		}

		av, err := convertToAttributeValue(m)
		Expect(err).NotTo(HaveOccurred())
		Expect(av.M).NotTo(BeNil())
		Expect(av.M).To(HaveLen(4))

		Expect(av.M["string_field"].S).NotTo(BeNil())
		Expect(*av.M["string_field"].S).To(Equal("value"))
		Expect(av.M["number_field"].N).NotTo(BeNil())
		Expect(*av.M["number_field"].N).To(Equal("123"))
		Expect(av.M["bool_field"].BOOL).NotTo(BeNil())
		Expect(*av.M["bool_field"].BOOL).To(BeFalse())
		Expect(av.M["null_field"].NULL).NotTo(BeNil())
		Expect(*av.M["null_field"].NULL).To(BeTrue())
	})

	It("should convert a string set", func() {
		ss := []string{"item1", "item2", "item3"}
		av, err := convertToAttributeValue(ss)
		Expect(err).NotTo(HaveOccurred())
		Expect(av.SS).NotTo(BeNil())
		Expect(av.SS).To(HaveLen(3))
		Expect(*av.SS[0]).To(Equal("item1"))
		Expect(*av.SS[1]).To(Equal("item2"))
		Expect(*av.SS[2]).To(Equal("item3"))
	})

	It("should error on empty string set", func() {
		ss := []string{}
		av, err := convertToAttributeValue(ss)
		Expect(err).To(HaveOccurred())
		Expect(av).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("string set cannot be empty"))
	})

	It("should convert nested structures", func() {
		nested := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": []interface{}{
					map[string]interface{}{
						"level3": "deep-value",
					},
					"list-item",
				},
			},
			"simple": "value",
		}

		av, err := convertToAttributeValue(nested)
		Expect(err).NotTo(HaveOccurred())
		Expect(av.M).NotTo(BeNil())

		level1, ok := av.M["level1"]
		Expect(ok).To(BeTrue())
		Expect(level1.M).NotTo(BeNil())

		level2, ok := level1.M["level2"]
		Expect(ok).To(BeTrue())
		Expect(level2.L).NotTo(BeNil())
		Expect(level2.L).To(HaveLen(2))

		level3Map := level2.L[0]
		Expect(level3Map.M).NotTo(BeNil())
		level3Value := level3Map.M["level3"]
		Expect(level3Value.S).NotTo(BeNil())
		Expect(*level3Value.S).To(Equal("deep-value"))

		listItem := level2.L[1]
		Expect(listItem.S).NotTo(BeNil())
		Expect(*listItem.S).To(Equal("list-item"))

		simple := av.M["simple"]
		Expect(simple.S).NotTo(BeNil())
		Expect(*simple.S).To(Equal("value"))
	})

	It("should error on unsupported type", func() {
		type CustomType struct {
			Field string
		}
		av, err := convertToAttributeValue(CustomType{Field: "value"})
		Expect(err).To(HaveOccurred())
		Expect(av).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("unsupported type"))
	})

	DescribeTable("should convert number types",
		func(input interface{}, expected string) {
			av, err := convertToAttributeValue(input)
			Expect(err).NotTo(HaveOccurred())
			Expect(av.N).NotTo(BeNil())
			Expect(*av.N).To(Equal(expected))
		},
		Entry("int", 123, "123"),
		Entry("int64", int64(456), "456"),
		Entry("float64 integer", float64(789), "789"),
		Entry("float64 decimal", 123.45, "123.45"),
		Entry("float64 negative", -67.89, "-67.89"),
		Entry("int negative", -100, "-100"),
	)
})

var _ = Describe("convertToAttributeValueMap", func() {
	It("should convert a full item map", func() {
		item := map[string]interface{}{
			"id":     "test-123",
			"name":   "Test Item",
			"count":  int64(42),
			"active": true,
			"tags":   []interface{}{"tag1", "tag2"},
			"metadata": map[string]interface{}{
				"created": "2024-01-01",
			},
		}

		result, err := convertToAttributeValueMap(item)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(HaveLen(6))

		Expect(result["id"].S).NotTo(BeNil())
		Expect(*result["id"].S).To(Equal("test-123"))
		Expect(result["name"].S).NotTo(BeNil())
		Expect(*result["name"].S).To(Equal("Test Item"))
		Expect(result["count"].N).NotTo(BeNil())
		Expect(*result["count"].N).To(Equal("42"))
		Expect(result["active"].BOOL).NotTo(BeNil())
		Expect(*result["active"].BOOL).To(BeTrue())
		Expect(result["tags"].L).NotTo(BeNil())
		Expect(result["tags"].L).To(HaveLen(2))
		Expect(result["metadata"].M).NotTo(BeNil())
		Expect(result["metadata"].M).To(HaveLen(1))
	})

	It("should error on unsupported type in map", func() {
		type UnsupportedType struct {
			Field string
		}
		item := map[string]interface{}{
			"valid_field":   "valid",
			"invalid_field": UnsupportedType{Field: "value"},
		}

		result, err := convertToAttributeValueMap(item)
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("failed to convert attribute"))
	})
})

var _ = Describe("UpdateItem", func() {
	It("should update an item successfully", func() {
		testClient := newTestClientWithPut()

		key := map[string]interface{}{"id": "test-123"}
		updates := map[string]interface{}{
			"name":   "Updated Name",
			"count":  int64(100),
			"active": false,
		}

		testClient.mockAPI.On("UpdateItem", mock.AnythingOfType("*dynamodb.UpdateItemInput")).
			Return(&dynamodb.UpdateItemOutput{}, nil)

		err := testClient.UpdateItem("test-table", key, updates)
		Expect(err).NotTo(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should update with nested structures", func() {
		testClient := newTestClientWithPut()

		key := map[string]interface{}{"id": "test-456"}
		updates := map[string]interface{}{
			"metadata": map[string]interface{}{
				"updated": "2024-01-02",
				"version": int64(2),
			},
			"tags": []interface{}{"tag1", "tag2", "tag3", "tag4"},
		}

		testClient.mockAPI.On("UpdateItem", mock.AnythingOfType("*dynamodb.UpdateItemInput")).
			Return(&dynamodb.UpdateItemOutput{}, nil)

		err := testClient.UpdateItem("test-table", key, updates)
		Expect(err).NotTo(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should update with null value", func() {
		testClient := newTestClientWithPut()

		key := map[string]interface{}{"id": "test-789"}
		updates := map[string]interface{}{
			"description": nil,
			"status":      "inactive",
		}

		testClient.mockAPI.On("UpdateItem", mock.AnythingOfType("*dynamodb.UpdateItemInput")).
			Return(&dynamodb.UpdateItemOutput{}, nil)

		err := testClient.UpdateItem("test-table", key, updates)
		Expect(err).NotTo(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should return error on DynamoDB failure", func() {
		testClient := newTestClientWithPut()

		key := map[string]interface{}{"id": "test-error"}
		updates := map[string]interface{}{"name": "Error Update"}

		testClient.mockAPI.On("UpdateItem", mock.AnythingOfType("*dynamodb.UpdateItemInput")).
			Return((*dynamodb.UpdateItemOutput)(nil), errors.New("dynamodb error"))

		err := testClient.UpdateItem("test-table", key, updates)
		Expect(err).To(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})

	It("should error on invalid key type", func() {
		testClient := newTestClientWithPut()

		type UnsupportedType struct{ Field string }
		key := map[string]interface{}{"id": UnsupportedType{Field: "invalid"}}
		updates := map[string]interface{}{"name": "Test"}

		err := testClient.UpdateItem("test-table", key, updates)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Failed to convert key to DynamoDB format"))
	})

	It("should error on invalid update value", func() {
		testClient := newTestClientWithPut()

		type UnsupportedType struct{ Field string }
		key := map[string]interface{}{"id": "test-123"}
		updates := map[string]interface{}{"invalid_field": UnsupportedType{Field: "invalid"}}

		err := testClient.UpdateItem("test-table", key, updates)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Failed to build update expression"))
	})

	It("should error on empty updates", func() {
		testClient := newTestClientWithPut()

		key := map[string]interface{}{"id": "test-123"}
		updates := map[string]interface{}{}

		err := testClient.UpdateItem("test-table", key, updates)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Failed to build update expression"))
	})

	It("should update with composite key", func() {
		testClient := newTestClientWithPut()

		key := map[string]interface{}{
			"pk": "partition-key",
			"sk": "sort-key",
		}
		updates := map[string]interface{}{
			"name":  "Updated Item",
			"count": int64(50),
		}

		testClient.mockAPI.On("UpdateItem", mock.AnythingOfType("*dynamodb.UpdateItemInput")).
			Return(&dynamodb.UpdateItemOutput{}, nil)

		err := testClient.UpdateItem("test-table", key, updates)
		Expect(err).NotTo(HaveOccurred())
		testClient.mockAPI.AssertExpectations(GinkgoT())
	})
})

var _ = Describe("buildUpdateExpression", func() {
	It("should build expression for single attribute", func() {
		updates := map[string]interface{}{"name": "Test Name"}

		expression, names, values, err := buildUpdateExpression(updates)
		Expect(err).NotTo(HaveOccurred())
		Expect(expression).To(ContainSubstring("SET"))
		Expect(names).To(HaveLen(1))
		Expect(values).To(HaveLen(1))
		Expect(expression).To(ContainSubstring("#attr0"))
		Expect(expression).To(ContainSubstring(":val0"))
		Expect(*names["#attr0"]).To(Equal("name"))
		Expect(values[":val0"].S).NotTo(BeNil())
		Expect(*values[":val0"].S).To(Equal("Test Name"))
	})

	It("should build expression for multiple attributes", func() {
		updates := map[string]interface{}{
			"name":   "Test Name",
			"count":  int64(42),
			"active": true,
		}

		expression, names, values, err := buildUpdateExpression(updates)
		Expect(err).NotTo(HaveOccurred())
		Expect(expression).To(ContainSubstring("SET"))
		Expect(names).To(HaveLen(3))
		Expect(values).To(HaveLen(3))
		Expect(expression).To(ContainSubstring("#attr"))
		Expect(expression).To(ContainSubstring(":val"))

		foundNames := make(map[string]bool)
		for _, name := range names {
			foundNames[*name] = true
		}
		Expect(foundNames["name"]).To(BeTrue())
		Expect(foundNames["count"]).To(BeTrue())
		Expect(foundNames["active"]).To(BeTrue())
	})

	It("should error on empty updates", func() {
		updates := map[string]interface{}{}

		expression, names, values, err := buildUpdateExpression(updates)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no updates provided"))
		Expect(expression).To(BeEmpty())
		Expect(names).To(BeNil())
		Expect(values).To(BeNil())
	})

	It("should error on invalid value", func() {
		type UnsupportedType struct{ Field string }
		updates := map[string]interface{}{
			"valid_field":   "valid",
			"invalid_field": UnsupportedType{Field: "invalid"},
		}

		expression, names, values, err := buildUpdateExpression(updates)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to convert attribute"))
		Expect(expression).To(BeEmpty())
		Expect(names).To(BeNil())
		Expect(values).To(BeNil())
	})

	It("should handle reserved words via expression attribute names", func() {
		updates := map[string]interface{}{
			"name":   "Test",
			"status": "active",
			"count":  int64(10),
		}

		expression, names, values, err := buildUpdateExpression(updates)
		Expect(err).NotTo(HaveOccurred())
		Expect(expression).To(ContainSubstring("SET"))
		Expect(names).To(HaveLen(3))
		Expect(values).To(HaveLen(3))
		Expect(expression).To(ContainSubstring("#attr"))
		Expect(expression).To(ContainSubstring(":val"))
	})

	It("should handle nested structures", func() {
		updates := map[string]interface{}{
			"metadata": map[string]interface{}{
				"created": "2024-01-01",
				"version": int64(1),
			},
			"tags": []interface{}{"tag1", "tag2"},
		}

		expression, names, values, err := buildUpdateExpression(updates)
		Expect(err).NotTo(HaveOccurred())
		Expect(expression).To(ContainSubstring("SET"))
		Expect(names).To(HaveLen(2))
		Expect(values).To(HaveLen(2))

		var foundMetadata, foundTags bool
		for placeholder, name := range names {
			if *name == "metadata" {
				foundMetadata = true
				Expect(values[":val"+placeholder[5:]].M).NotTo(BeNil())
			}
			if *name == "tags" {
				foundTags = true
				Expect(values[":val"+placeholder[5:]].L).NotTo(BeNil())
			}
		}
		Expect(foundMetadata).To(BeTrue())
		Expect(foundTags).To(BeTrue())
	})
})
