package mongoManager

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// TransactionFunc defines a function to be executed within a transaction.
// It receives a context with the session bound - all MongoDB operations
// using this context will automatically participate in the transaction.
//
// Return an error to abort the transaction, or nil to commit.
type TransactionFunc func(ctx context.Context) (interface{}, error)

// RunTransaction executes the provided function within a MongoDB transaction.
// All MongoDB operations using the provided context will participate in the transaction.
//
// Features:
//   - Automatic retry for TransientTransactionError (whole transaction retried)
//   - Automatic retry for UnknownTransactionCommitResult (commit retried)
//   - Automatic session cleanup (defer EndSession)
//   - Automatic abort on error
//
// Parameters:
//   - ctx: Base context for the operation
//   - txnFunc: Function to execute within the transaction
//
// Returns:
//   - interface{}: Result from txnFunc on success
//   - error: nil on success, or the error if transaction fails
//
// Example:
//
//	result, err := mongoManager.RunTransaction(ctx, func(txCtx context.Context) (interface{}, error) {
//	    user, err := userRepo.Create(txCtx, userData)
//	    if err != nil {
//	        return nil, err // Aborts transaction
//	    }
//	    membership, err := membershipRepo.Create(txCtx, membershipData)
//	    if err != nil {
//	        return nil, err // Aborts transaction, user creation rolled back
//	    }
//	    return user, nil // Commits transaction
//	})
func (m *MongoManager) RunTransaction(ctx context.Context, txnFunc TransactionFunc) (interface{}, error) {
	if !m.IsConnected() {
		return nil, ErrNotConnected
	}

	session, err := m.NewSession(ctx)
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	return m.runTransactionWithRetry(ctx, session, txnFunc)
}

// NewSession creates a new MongoDB session for transaction support.
// The caller is responsible for calling session.EndSession() when done.
//
// Example:
//
//	session, err := mongoManager.NewSession(ctx)
//	if err != nil {
//	    return err
//	}
//	defer session.EndSession(ctx)
func (m *MongoManager) NewSession(ctx context.Context) (*mongo.Session, error) {
	if !m.IsConnected() {
		return nil, ErrNotConnected
	}

	session, err := m.client.StartSession()
	if err != nil {
		logger.Error(err, "[INFRA:MONGODB] Failed to start session")
		return nil, err
	}

	return session, nil
}

// RunTransactionWithClient is a standalone function for running transactions with just a client.
// This is useful for packages that don't have access to MongoManager but have a *mongo.Client.
//
// Model[T] uses this to delegate transaction handling to the manager package.
func RunTransactionWithClient(ctx context.Context, client *mongo.Client, txnFunc TransactionFunc) (interface{}, error) {
	if client == nil {
		return nil, ErrNotConnected
	}

	session, err := client.StartSession()
	if err != nil {
		logger.Error(err, "[INFRA:MONGODB] Failed to start session")
		return nil, err
	}
	defer session.EndSession(ctx)

	return runTransactionWithRetryInternal(ctx, session, txnFunc)
}
