package authn

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/wjbbeyond/guardrail/internal/config"
)

type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewOIDCVerifier(ctx context.Context, cfg config.OIDCConfig) (*OIDCVerifier, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}
	return &OIDCVerifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (Token, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Token{}, fmt.Errorf("verify id token: %w", err)
	}
	claims := make(map[string]any)
	if err := idToken.Claims(&claims); err != nil {
		return Token{}, fmt.Errorf("decode id token claims: %w", err)
	}
	return Token{Subject: idToken.Subject, Claims: claims}, nil
}
