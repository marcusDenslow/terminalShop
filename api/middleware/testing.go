package middleware

import "context"

// ContextWithUserID returns a context with the given user ID set.
// This is intended for use in tests only.
func ContextWithUserID(ctx context.Context, id uint) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}
