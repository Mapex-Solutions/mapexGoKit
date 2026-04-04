package natsModel

import "errors"

var ErrMissingHandler = errors.New("handler function must be provided")
var ErrMissingSubject = errors.New("subject must be provided")

// Retry and DLQ errors
var ErrMaxRetriesExceeded = errors.New("max retries exceeded")
var ErrDLQPublishFailed = errors.New("failed to publish message to DLQ")
var ErrMessageMetadataFailed = errors.New("failed to get message metadata")
var ErrDLQNotConfigured = errors.New("DLQ policy not configured")
