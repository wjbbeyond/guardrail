package authn

import "context"

const DefaultTenantID = "default"

type Identity struct {
	TenantID string
	Subject  string
	Method   string
	Admin    bool
}

type identityKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	if identity.TenantID == "" {
		identity.TenantID = DefaultTenantID
	}
	return context.WithValue(ctx, identityKey{}, identity)
}

func FromContext(ctx context.Context) Identity {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	if !ok || identity.TenantID == "" {
		return Identity{TenantID: DefaultTenantID, Subject: DefaultTenantID, Method: "anonymous"}
	}
	return identity
}
