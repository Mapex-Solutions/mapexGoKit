package mongoManager

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

func (m *MongoManager) connect() error {
	clientOpts := options.Client().ApplyURI(m.cfg.URI)

	// Default: plain map[string]interface{} for interface{} fields.
	// DefaultDocumentMap returns map[string]any (not bson.M named type),
	// so standard Go type assertions like val.(map[string]interface{}) work.
	// Only use bson.D when explicitly requested via UseBsonD.
	if !m.cfg.UseBsonD {
		clientOpts.SetBSONOptions(&options.BSONOptions{
			DefaultDocumentMap: true,
		})
	}

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		logger.Error(err, "[INFRA:MONGODB] Connect error")
		m.isConnected.Store(false)
		return err
	}

	ctxPing, cancelPing := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelPing()

	if err := client.Ping(ctxPing, nil); err != nil {
		logger.Error(err, "[INFRA:MONGODB] Ping failed")
		m.isConnected.Store(false)
		return err
	}

	m.mu.Lock()
	m.client = client
	m.mu.Unlock()

	m.isConnected.Store(true)
	logger.Info("[INFRA:MONGODB] Connected")
	return nil
}

func (m *MongoManager) startMonitor() {
	ticker := time.NewTicker(m.cfg.MonitorInterval * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.RLock()
		client := m.client
		m.mu.RUnlock()

		if client == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		start := time.Now()
		err := client.Ping(ctx, nil)
		latency := time.Since(start)
		cancel()

		if err != nil {
			logger.Error(err, "[INFRA:MONGODB] Connection lost")
			m.isConnected.Store(false)
		} else {
			m.lastLatency.Store(latency.Milliseconds())
			m.isConnected.Store(true)
		}
	}
}

//
// TRANSACTION INTERNALS
//

// runTransactionWithRetry delegates to the internal implementation.
func (m *MongoManager) runTransactionWithRetry(
	ctx context.Context,
	session *mongo.Session,
	txnFunc TransactionFunc,
) (interface{}, error) {
	return runTransactionWithRetryInternal(ctx, session, txnFunc)
}

// runTransactionWithRetryInternal is the core transaction logic used by both
// MongoManager.RunTransaction and RunTransactionWithClient.
func runTransactionWithRetryInternal(
	ctx context.Context,
	session *mongo.Session,
	txnFunc TransactionFunc,
) (interface{}, error) {

	for {
		if err := session.StartTransaction(); err != nil {
			logger.Error(err, "[INFRA:MONGODB] Failed to start transaction")
			return nil, err
		}

		sessCtx := mongo.NewSessionContext(ctx, session)
		result, err := txnFunc(sessCtx)

		if err != nil {
			if abortErr := session.AbortTransaction(ctx); abortErr != nil {
				logger.Error(abortErr, "[INFRA:MONGODB] Failed to abort transaction")
			}

			if hasErrorLabel(err, "TransientTransactionError") {
				logger.Warn("[INFRA:MONGODB] TransientTransactionError, retrying transaction...")
				continue
			}

			logger.Error(err, "[INFRA:MONGODB] Transaction function failed")
			return nil, err
		}

		if err := commitWithRetryInternal(ctx, session); err != nil {
			if hasErrorLabel(err, "TransientTransactionError") {
				logger.Warn("[INFRA:MONGODB] TransientTransactionError during commit, retrying transaction...")
				continue
			}

			if abortErr := session.AbortTransaction(ctx); abortErr != nil {
				logger.Error(abortErr, "[INFRA:MONGODB] Failed to abort transaction after commit failure")
			}
			return nil, err
		}

		logger.Debug("[INFRA:MONGODB] Transaction committed successfully")
		return result, nil
	}
}

// commitWithRetryInternal is the core commit logic with retry.
func commitWithRetryInternal(ctx context.Context, session *mongo.Session) error {
	for {
		err := session.CommitTransaction(ctx)
		if err == nil {
			return nil
		}

		if hasErrorLabel(err, "UnknownTransactionCommitResult") {
			logger.Warn("[INFRA:MONGODB] UnknownTransactionCommitResult, retrying commit...")
			continue
		}

		logger.Error(err, "[INFRA:MONGODB] Failed to commit transaction")
		return err
	}
}

// hasErrorLabel checks if a MongoDB error has a specific error label.
func hasErrorLabel(err error, label string) bool {
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr.HasErrorLabel(label)
	}

	var writeErr mongo.WriteException
	if errors.As(err, &writeErr) {
		return writeErr.HasErrorLabel(label)
	}

	return false
}
