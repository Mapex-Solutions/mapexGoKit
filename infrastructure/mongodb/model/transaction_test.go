package mongoModel

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TestDocument is a simple document type for testing
type TestDocument struct {
	ID    bson.ObjectID `bson:"_id,omitempty"`
	Name  string        `bson:"name"`
	Value int           `bson:"value"`
}

// TestNewSession tests session creation
func TestNewSession(t *testing.T) {
	model := setupTestModel[TestDocument](t)
	if model == nil {
		t.Skip("Skipping test: MongoDB not available")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := model.NewSession(ctx)
	if err != nil {
		t.Fatalf("NewSession() returned error: %v", err)
	}

	if session == nil {
		t.Fatal("NewSession() returned nil session")
	}

	// Clean up
	session.EndSession(ctx)
}

// TestRunTransaction_Success tests a successful transaction
func TestRunTransaction_Success(t *testing.T) {
	model := setupTestModel[TestDocument](t)
	if model == nil {
		t.Skip("Skipping test: MongoDB not available")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run a transaction that inserts a document
	testDoc := TestDocument{Name: "test", Value: 123}

	result, err := model.RunTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		created, err := model.CreateOne(sessCtx, &testDoc)
		if err != nil {
			return nil, err
		}
		return created, nil
	})

	if err != nil {
		t.Fatalf("RunTransaction() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("RunTransaction() returned nil result")
	}

	// Verify the document was inserted
	createdDoc := result.(*TestDocument)
	found, err := model.FindByID(ctx, createdDoc.ID.Hex())
	if err != nil {
		t.Fatalf("Document not found after transaction: %v", err)
	}

	if found.Value != 123 {
		t.Fatalf("Expected value 123, got: %v", found.Value)
	}

	// Cleanup
	model.DeleteByID(ctx, createdDoc.ID.Hex())
}

// TestRunTransaction_Rollback tests that a transaction is rolled back on error
func TestRunTransaction_Rollback(t *testing.T) {
	model := setupTestModel[TestDocument](t)
	if model == nil {
		t.Skip("Skipping test: MongoDB not available")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run a transaction that fails after inserting
	testDoc := TestDocument{Name: "should_rollback", Value: 456}
	expectedErr := errors.New("intentional error for testing")

	_, err := model.RunTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		// Insert a document
		_, err := model.CreateOne(sessCtx, &testDoc)
		if err != nil {
			return nil, err
		}

		// Return an error to trigger rollback
		return nil, expectedErr
	})

	if err == nil {
		t.Fatal("Expected error from transaction")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf("Expected intentional error, got: %v", err)
	}

	// Verify the document was NOT inserted (rolled back) using direct collection access
	count, err := model.DIRECT().CountDocuments(ctx, Map{"name": "should_rollback"})
	if err != nil {
		t.Fatalf("CountDocuments() returned error: %v", err)
	}

	if count != 0 {
		t.Fatalf("Expected 0 documents (rollback), got: %d", count)
	}
}

// TestRunTransaction_MultipleOperations tests a transaction with multiple operations
func TestRunTransaction_MultipleOperations(t *testing.T) {
	model := setupTestModel[TestDocument](t)
	if model == nil {
		t.Skip("Skipping test: MongoDB not available")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run a transaction with multiple inserts
	doc1 := TestDocument{Name: "multi_1", Value: 100}
	doc2 := TestDocument{Name: "multi_2", Value: 200}

	result, err := model.RunTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		// Insert first document
		created1, err := model.CreateOne(sessCtx, &doc1)
		if err != nil {
			return nil, err
		}

		// Insert second document
		created2, err := model.CreateOne(sessCtx, &doc2)
		if err != nil {
			return nil, err
		}

		return []bson.ObjectID{created1.ID, created2.ID}, nil
	})

	if err != nil {
		t.Fatalf("RunTransaction() returned error: %v", err)
	}

	ids := result.([]bson.ObjectID)
	if len(ids) != 2 {
		t.Fatalf("Expected 2 IDs, got: %d", len(ids))
	}

	// Verify both documents were inserted using direct collection access
	count, _ := model.DIRECT().CountDocuments(ctx, Map{"name": Map{"$in": []string{"multi_1", "multi_2"}}})
	if count != 2 {
		t.Fatalf("Expected 2 documents, got: %d", count)
	}

	// Cleanup
	for _, id := range ids {
		model.DeleteByID(ctx, id.Hex())
	}
}

// TestRunTransactionWithRetry_ManualSession tests using RunTransactionWithRetry with a manual session
func TestRunTransactionWithRetry_ManualSession(t *testing.T) {
	model := setupTestModel[TestDocument](t)
	if model == nil {
		t.Skip("Skipping test: MongoDB not available")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create session manually
	session, err := model.NewSession(ctx)
	if err != nil {
		t.Fatalf("NewSession() returned error: %v", err)
	}
	defer session.EndSession(ctx)

	// Run transaction with the manual session
	testDoc := TestDocument{Name: "manual_session_test", Value: 789}

	result, err := model.RunTransactionWithRetry(ctx, session, func(sessCtx context.Context) (interface{}, error) {
		created, err := model.CreateOne(sessCtx, &testDoc)
		if err != nil {
			return nil, err
		}
		return created, nil
	})

	if err != nil {
		t.Fatalf("RunTransactionWithRetry() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("RunTransactionWithRetry() returned nil result")
	}

	// Verify the document was inserted
	createdDoc := result.(*TestDocument)
	found, err := model.FindByID(ctx, createdDoc.ID.Hex())
	if err != nil {
		t.Fatalf("Document not found after transaction: %v", err)
	}

	if found.Name != "manual_session_test" {
		t.Fatalf("Expected name 'manual_session_test', got: %v", found.Name)
	}

	// Cleanup
	model.DeleteByID(ctx, createdDoc.ID.Hex())
}

// TestHasErrorLabel tests the error label checking function
func TestHasErrorLabel(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		label    string
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			label:    "TransientTransactionError",
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("some error"),
			label:    "TransientTransactionError",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasErrorLabel(tt.err, tt.label)
			if result != tt.expected {
				t.Errorf("hasErrorLabel() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// setupTestModel creates a Model for testing
// Returns nil if MongoDB is not available
func setupTestModel[T any](t *testing.T) *Model[T] {
	t.Helper()

	// Try to connect to a local MongoDB instance
	uri := "mongodb://localhost:27017/?replicaSet=rs0"
	dbName := "test_transactions"
	collName := "test_docs_" + bson.NewObjectID().Hex()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try with replica set first
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		// Try without replica set
		client, err = mongo.Connect(options.Client().ApplyURI("mongodb://localhost:27017"))
		if err != nil {
			return nil
		}
	}

	// Check connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil
	}

	db := client.Database(dbName)
	model := New[T](db, collName, Config{})

	// Register cleanup
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		db.Collection(collName).Drop(cleanupCtx)
		client.Disconnect(cleanupCtx)
	})

	return model
}

// Note: For transactions to work in MongoDB, you need:
// 1. MongoDB 4.0+ with a replica set configuration
// 2. Or MongoDB 4.2+ with sharded cluster
//
// For local development/testing, you can start a single-node replica set:
//   mongod --replSet rs0 --bind_ip localhost
//   mongosh --eval "rs.initiate()"
//
// Or use Docker:
//   docker run -d --name mongo -p 27017:27017 mongo:7 --replSet rs0
//   docker exec mongo mongosh --eval "rs.initiate()"
