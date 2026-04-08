package reqctx

import (
	"context"

	"go.e64ec.com/booksmk/internal/store"
)

type contextKey int

const userKey contextKey = iota

func WithUser(ctx context.Context, u store.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func User(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userKey).(store.User)
	return u, ok
}
