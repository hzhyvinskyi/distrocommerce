// Package jwt provides RSA key management and JWT signing for identity-service.
// Only identity-service should import this package — it holds the private key.
//
// bff-web service uses pkg/auth/jwks instead, which only needs the public key
// fetched from the JWKS endpoint.
package jwt

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/hzhyvinskyi/distrocommerce/pkg/auth"
)

// RSAManager handles RS256 JWT signing and public key exposure via JWKS.
// Holds the private key — must only be used by identity-service.
type RSAManager struct {
	privateKey        *rsa.PrivateKey
	publicKey         *rsa.PublicKey
	keyID             string
	accessExpiration  time.Duration
	refreshExpiration time.Duration
	issuer            string
}

// NewRSAManager creates a Manager from a PEM-encoded RSA private key.
// The public key is derived from the private key.
// kid = SHA256(DER(publicKey))[:8] — stable across restarts.
func NewRSAManager(privateKeyPEM []byte, accessExp, refreshExp time.Duration, issuer string) (*RSAManager, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	var (
		err        error
		privateKey *rsa.PrivateKey
	)

	switch block.Type {
	case "RSA PRIVATE KEY":
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, e := x509.ParsePKCS8PrivateKey(block.Bytes)
		if e != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", e)
		}

		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &RSAManager{
		privateKey:        privateKey,
		publicKey:         &privateKey.PublicKey,
		keyID:             deriveKeyID(&privateKey.PublicKey),
		accessExpiration:  accessExp,
		refreshExpiration: refreshExp,
		issuer:            issuer,
	}, nil
}

func (m *RSAManager) GenerateTokenPair(identityID, email string, roles []string) (*auth.TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(m.accessExpiration)

	accessToken, err := m.sign(identityID, email, roles, accessExp)
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	refreshExp := now.Add(m.refreshExpiration)
	refreshToken, err := m.sign(identityID, email, roles, refreshExp)
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &auth.TokenPair{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(m.accessExpiration.Seconds()),
		ExpiresAt:    accessExp.Unix(),
		RefreshToken: refreshToken,
	}, nil
}

func (m *RSAManager) sign(identityID, email string, roles []string, exp time.Time) (string, error) {
	claims := &auth.Claims{
		UserID: identityID,
		Email:  email,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			ExpiresAt: jwt.NewNumericDate(exp),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        m.keyID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = m.keyID

	return token.SignedString(m.privateKey)
}

// ValidateToken validates an RS256 JWT using the local private key's public key.
// Used by identity-service to validate refresh tokens.
func (m *RSAManager) ValidateToken(tokenStr string) (*auth.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &auth.Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return m.publicKey, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, auth.ErrExpiredToken
		}

		return nil, auth.ErrInvalidToken
	}

	claims, ok := token.Claims.(*auth.Claims)
	if !ok || !token.Valid {
		return nil, auth.ErrInvalidToken
	}

	return claims, nil
}

// deriveKeyID = SHA256(DER(publicKey))[:8 hex chars]
// Deterministic — same key always produces same kid across restarts.
func deriveKeyID(pub *rsa.PublicKey) string {
	der := mustMarshalPublicKey(pub)
	h := sha256.Sum256(der)

	return fmt.Sprintf("%x", h[:4])
}

func mustMarshalPublicKey(key *rsa.PublicKey) []byte {
	b, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		panic(err)
	}

	return b
}
