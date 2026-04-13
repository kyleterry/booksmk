package auth

import (
	"context"

	"go.e64ec.com/booksmk/internal/store"
)

type contextKey struct{}

// NewContextWithUser returns a new context with u stored as the authenticated user.
func NewContextWithUser(ctx context.Context, u store.User) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}

// UserFromContext returns the authenticated user stored in ctx and whether one was present.
func UserFromContext(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(contextKey{}).(store.User)
	return u, ok
}
