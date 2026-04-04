package mongoModel

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"

	manager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb/manager"
)

// TransactionFunc defines a function to be executed within a transaction.
// It receives a context with the session bound - all MongoDB operations
// using this context will automatically participate in the transaction.
//
// Return an error to abort the transaction, or nil to commit.
type TransactionFunc = manager.TransactionFunc

// NewSession creates a new MongoDB session for transaction support.
// The caller is responsible for calling session.EndSession() when done.
//
// Delegates to manager package for centralized transaction handling.
func (m *Model[T]) NewSession(ctx context.Context) (*mongo.Session, error) {
	client := m.col.Database().Client()
	if client == nil {
		return nil, ErrNotConnected
	}

	return client.StartSession()
}

// RunTransaction is a convenience method that creates a session, runs the transaction
// with retry logic, and cleans up the session automatically.
//
// Delegates to manager.RunTransactionWithClient for centralized transaction handling.
//
// Example:
//
//	result, err := userModel.RunTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
//	    user, err := userModel.CreateOne(sessCtx, userData)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return user, nil
//	})
func (m *Model[T]) RunTransaction(ctx context.Context, txnFunc TransactionFunc) (interface{}, error) {
	client := m.col.Database().Client()
	return manager.RunTransactionWithClient(ctx, client, txnFunc)
}

// RunTransactionWithRetry executes a transaction function with automatic retry for transient errors.
//
// Delegates to manager.RunTransactionWithClient for centralized transaction handling.
// Prefer using RunTransaction() which handles session creation automatically.
func (m *Model[T]) RunTransactionWithRetry(
	ctx context.Context,
	session *mongo.Session,
	txnFunc TransactionFunc,
) (interface{}, error) {
	client := m.col.Database().Client()
	return manager.RunTransactionWithClient(ctx, client, txnFunc)
}

// CommitWithRetry commits a transaction with automatic retry for transient errors.
//
// Deprecated: Use RunTransaction() instead which handles commit automatically.
// Kept for backwards compatibility.
func (m *Model[T]) CommitWithRetry(ctx context.Context, session *mongo.Session) error {
	return session.CommitTransaction(ctx)
}
