package mock

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	identity "github.com/company/pda-backend/internal/identity/domain"
	"github.com/company/pda-backend/internal/identity/ports"
	"github.com/google/uuid"
)

type TokenProvider struct {
	secret  []byte
	ttl     time.Duration
	mu      sync.RWMutex
	revoked map[string]struct{}
}
type claims struct {
	Subject   string `json:"sub"`
	TokenID   string `json:"jti"`
	ExpiresAt int64  `json:"exp"`
	Issuer    string `json:"iss"`
	Kind      string `json:"kind"`
}

func NewTokenProvider(secret string, ttl time.Duration) (*TokenProvider, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("mock token secret must contain at least 32 bytes")
	}
	return &TokenProvider{secret: []byte(secret), ttl: ttl, revoked: map[string]struct{}{}}, nil
}

func (p *TokenProvider) Issue(operator identity.Operator, now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT", "mock": "true"})
	payload, _ := json.Marshal(claims{Subject: operator.ID, TokenID: uuid.NewString(), ExpiresAt: now.Add(p.ttl).Unix(), Issuer: "pda-local-mock", Kind: "access"})
	unsigned := encode(header) + "." + encode(payload)
	return unsigned + "." + p.sign(unsigned), nil
}

func (p *TokenProvider) IssueRefresh(operator identity.Operator, now time.Time) (string, error) {
	return p.issue(operator, now, "refresh", p.ttl*24)
}

func (p *TokenProvider) issue(operator identity.Operator, now time.Time, kind string, ttl time.Duration) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT", "mock": "true"})
	payload, _ := json.Marshal(claims{Subject: operator.ID, TokenID: uuid.NewString(), ExpiresAt: now.Add(ttl).Unix(), Issuer: "pda-local-mock", Kind: kind})
	unsigned := encode(header) + "." + encode(payload)
	return unsigned + "." + p.sign(unsigned), nil
}

func (p *TokenProvider) ValidateRefresh(token string, now time.Time) (ports.Claims, error) {
	return p.validate(token, now, "refresh")
}

func (p *TokenProvider) Validate(token string, now time.Time) (ports.Claims, error) {
	return p.validate(token, now, "access")
}

func (p *TokenProvider) validate(token string, now time.Time, kind string) (ports.Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || !hmac.Equal([]byte(parts[2]), []byte(p.sign(parts[0]+"."+parts[1]))) {
		return ports.Claims{}, fmt.Errorf("invalid mock token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ports.Claims{}, err
	}
	var parsed claims
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return ports.Claims{}, err
	}
	expires := time.Unix(parsed.ExpiresAt, 0)
	p.mu.RLock()
	_, revoked := p.revoked[parsed.TokenID]
	p.mu.RUnlock()
	if parsed.Issuer != "pda-local-mock" || parsed.Kind != kind || parsed.Subject == "" || parsed.TokenID == "" || revoked || !expires.After(now) {
		return ports.Claims{}, fmt.Errorf("expired or revoked mock token")
	}
	return ports.Claims{OperatorID: parsed.Subject, TokenID: parsed.TokenID, ExpiresAt: expires}, nil
}

func (p *TokenProvider) Revoke(token string) error {
	claims, err := p.validate(token, time.Unix(0, 0), "access")
	if err != nil {
		claims, err = p.validate(token, time.Unix(0, 0), "refresh")
	}
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.revoked[claims.TokenID] = struct{}{}
	return nil
}

func encode(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }
func (p *TokenProvider) sign(value string) string {
	mac := hmac.New(sha256.New, p.secret)
	_, _ = mac.Write([]byte(value))
	return encode(mac.Sum(nil))
}
