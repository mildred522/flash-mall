package authctx

import "context"

type identityKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	if ctx == nil {
		return Identity{}, false
	}
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok
}

func MustIdentity(ctx context.Context) Identity {
	identity, _ := IdentityFrom(ctx)
	return identity
}
