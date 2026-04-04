package mongoModel

import (
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Constants related to default behaviors and MongoDB operation modes.
const (
	// ReturnDocOld specifies that the original document should be returned before the update.
	ReturnDocOld = options.Before

	// ReturnDocNew specifies that the updated document should be returned after the update.
	ReturnDocNew = options.After

	// DefaultPage defines the default page number for offset-based pagination.
	DefaultPage int64 = 1

	// DefaultPerPage defines the default number of items per page for offset-based pagination.
	DefaultPerPage int64 = 25

	// MaxOffsetPerPage defines the upper limit of items that can be requested per page.
	MaxOffsetPerPage int64 = 300

	// MaxOffsetSkip defines the maximum number of documents that can be skipped using offset.
	MaxOffsetSkip int64 = 500
)
