package feeds

import "errors"

var (
	// Validation errors
	ErrEmptyDescription    = errors.New("description cannot be empty")
	ErrDescriptionTooLong  = errors.New("description too long (max 500 characters)")
	ErrNoFilters           = errors.New("at least one filter must be provided")
	ErrInvalidUUID         = errors.New("invalid UUID format")

	// Storage errors
	ErrFeedNotFound        = errors.New("feed not found")
	ErrFeedAlreadyExists   = errors.New("feed with this slug already exists")
	ErrStorageUnavailable  = errors.New("storage is unavailable")
	ErrInvalidFeedData     = errors.New("invalid feed data")
)
