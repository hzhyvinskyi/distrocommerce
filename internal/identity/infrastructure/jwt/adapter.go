package jwt

import authjwt "github.com/hzhyvinskyi/distrocommerce/pkg/auth/jwt"

// TokenManagerAdapter adapts authjwt.RSAManager to port.TokenManager.
// RS256: identity-service signs with private key, bff-web service validates with public key via JWKS.
type TokenManagerAdapter struct {
	m *authjwt.RSAManager
}

func NewTokenManagerAdapter(m *authjwt.RSAManager) *TokenManagerAdapter {
	return &TokenManagerAdapter{m: m}
}

func (a *TokenManagerAdapter) GenerateTokenPair(identityID, email string, roles []string) (accessToken, refreshToken string, expiresAt int64, err error) {
	pair, err := a.m.GenerateTokenPair(identityID, email, roles)
	if err != nil {
		return "", "", 0, err
	}

	return pair.AccessToken, pair.RefreshToken, pair.ExpiresAt, nil
}

func (a *TokenManagerAdapter) ValidateRefreshToken(token string) (identityID string, err error) {
	claims, err := a.m.ValidateToken(token)
	if err != nil {
		return "", err
	}

	return claims.UserID, nil
}
