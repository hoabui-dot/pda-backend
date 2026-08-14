package postgres

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	identity "github.com/company/pda-backend/internal/identity/domain"
	"github.com/company/pda-backend/internal/identity/ports"
	"github.com/google/uuid"
)

type SessionManager struct {
	Store                 *Store
	Secret                []byte
	Issuer, Audience      string
	AccessTTL, RefreshTTL time.Duration
	SigningMode           string
	KeyID                 string
	PrivateKey            *rsa.PrivateKey
	PublicKeys            map[string]*rsa.PublicKey
}

const absoluteRefreshLifetime = 30 * 24 * time.Hour

func NewRSASessionManager(store *Store, keys map[string]*rsa.PublicKey, privateKey *rsa.PrivateKey, keyID, issuer, audience string, accessTTL, refreshTTL time.Duration) (*SessionManager, error) {
	if store == nil || privateKey == nil || len(keys) == 0 || keys[keyID] == nil || keyID == "" || issuer == "" || audience == "" || accessTTL <= 0 || refreshTTL != absoluteRefreshLifetime {
		return nil, fmt.Errorf("invalid RSA identity configuration")
	}
	return &SessionManager{Store: store, Issuer: issuer, Audience: audience, AccessTTL: accessTTL, RefreshTTL: refreshTTL, SigningMode: "RS256", KeyID: keyID, PrivateKey: privateKey, PublicKeys: keys}, nil
}

func NewSessionManager(store *Store, secret []byte, issuer, audience string, accessTTL, refreshTTL time.Duration) (*SessionManager, error) {
	if store == nil || len(secret) < 32 || issuer == "" || audience == "" || accessTTL <= 0 || refreshTTL != absoluteRefreshLifetime {
		return nil, fmt.Errorf("invalid production identity configuration")
	}
	return &SessionManager{Store: store, Secret: secret, Issuer: issuer, Audience: audience, AccessTTL: accessTTL, RefreshTTL: refreshTTL}, nil
}

type tokenClaims struct {
	Issuer      string `json:"iss"`
	Audience    string `json:"aud"`
	Subject     string `json:"sub"`
	TokenID     string `json:"jti"`
	SessionID   string `json:"session_id"`
	OperatorID  string `json:"operator_id"`
	DeviceID    string `json:"device_id,omitempty"`
	WarehouseID string `json:"warehouse_id,omitempty"`
	IssuedAt    int64  `json:"iat"`
	NotBefore   int64  `json:"nbf"`
	ExpiresAt   int64  `json:"exp"`
}

func (m *SessionManager) Create(ctx context.Context, operator identity.Operator, deviceID, warehouseID string, now time.Time) (string, string, time.Time, time.Time, error) {
	if warehouseID != "" && !operator.CanAccessWarehouse(warehouseID) {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("warehouse denied")
	}
	if deviceID != "" && warehouseID != "" {
		ok, err := m.Store.IsRegistered(ctx, identity.DeviceRegistration{DeviceID: deviceID, OperatorID: operator.ID, WarehouseID: warehouseID})
		if err != nil || !ok {
			return "", "", time.Time{}, time.Time{}, fmt.Errorf("device not registered")
		}
	}
	tx, err := m.Store.Pool.Begin(ctx)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	defer tx.Rollback(ctx)
	sessionID := uuid.New()
	refreshID := uuid.New()
	familyID := uuid.New()
	expires := now.Add(m.AccessTTL)
	refreshExpires := now.Add(m.RefreshTTL)
	refresh, err := randomToken()
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_sessions(id,operator_id,device_code,warehouse_id,expires_at) VALUES($1,$2,$3,$4,$5)`, sessionID, operator.ID, nullString(deviceID), nullString(warehouseID), refreshExpires)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_refresh_tokens(id,session_id,token_hash,token_family_id,issued_at,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, refreshID, sessionID, tokenHash(refresh), familyID, now, refreshExpires)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	access, err := m.sign(tokenClaims{Issuer: m.Issuer, Audience: m.Audience, Subject: operator.ID, TokenID: uuid.NewString(), SessionID: sessionID.String(), OperatorID: operator.ID, DeviceID: deviceID, WarehouseID: warehouseID, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: expires.Unix()})
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	return access, refresh, expires, refreshExpires, nil
}
func (m *SessionManager) Authenticate(ctx context.Context, token string, now time.Time) (ports.Claims, error) {
	c, err := m.parse(token, now)
	if err != nil {
		return ports.Claims{}, err
	}
	var status string
	var expires time.Time
	err = m.Store.Pool.QueryRow(ctx, `SELECT status,expires_at FROM identity_sessions WHERE id=$1`, c.SessionID).Scan(&status, &expires)
	if err != nil || status != "ACTIVE" || !expires.After(now) {
		return ports.Claims{}, ports.ErrSessionRevoked
	}
	return ports.Claims{OperatorID: c.OperatorID, TokenID: c.TokenID, SessionID: c.SessionID, DeviceID: c.DeviceID, WarehouseID: c.WarehouseID, ExpiresAt: time.Unix(c.ExpiresAt, 0)}, nil
}
func (m *SessionManager) Refresh(ctx context.Context, raw, deviceID string, now time.Time) (string, string, time.Time, time.Time, string, string, string, error) {
	tx, err := m.Store.Pool.Begin(ctx)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, "", "", "", err
	}
	defer tx.Rollback(ctx)
	var id, sessionID uuid.UUID
	var operatorID, storedDevice, warehouse, sessionStatus, userStatus string
	var tokenExpires, sessionExpires time.Time
	var used, revoked *time.Time
	err = tx.QueryRow(ctx, `SELECT r.id,r.session_id,s.operator_id,COALESCE(s.device_code,''),COALESCE(s.warehouse_id,''),s.status,r.expires_at,s.expires_at,r.used_at,r.revoked_at,o.status FROM identity_refresh_tokens r JOIN identity_sessions s ON s.id=r.session_id JOIN identity_operators o ON o.id=s.operator_id WHERE r.token_hash=$1 FOR UPDATE`, tokenHash(raw)).Scan(&id, &sessionID, &operatorID, &storedDevice, &warehouse, &sessionStatus, &tokenExpires, &sessionExpires, &used, &revoked, &userStatus)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, "", "", "", ports.ErrRefreshTokenInvalid
	}
	if userStatus != "ACTIVE" {
		return "", "", time.Time{}, time.Time{}, "", "", "", ports.ErrUserDisabled
	}
	if used != nil || revoked != nil {
		_, _ = tx.Exec(ctx, `UPDATE identity_refresh_tokens SET reuse_detected_at=COALESCE(reuse_detected_at,$2) WHERE id=$1`, id, now)
		_, _ = tx.Exec(ctx, `UPDATE identity_sessions SET status='REVOKED',revoked_at=$2,revocation_reason='refresh_reuse' WHERE id=$1 AND status='ACTIVE'`, sessionID, now)
		_ = tx.Commit(ctx)
		return "", "", time.Time{}, time.Time{}, "", "", "", ports.ErrRefreshTokenReused
	}
	if sessionStatus != "ACTIVE" {
		return "", "", time.Time{}, time.Time{}, "", "", "", ports.ErrRefreshTokenRevoked
	}
	if !tokenExpires.After(now) || !sessionExpires.After(now) {
		return "", "", time.Time{}, time.Time{}, "", "", "", ports.ErrRefreshTokenExpired
	}
	if deviceID != "" && storedDevice != "" && deviceID != storedDevice {
		return "", "", time.Time{}, time.Time{}, "", "", "", ports.ErrDeviceMismatch
	}
	newRaw, err := randomToken()
	if err != nil {
		return "", "", time.Time{}, time.Time{}, "", "", "", err
	}
	newID := uuid.New()
	var familyID uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT token_family_id FROM identity_refresh_tokens WHERE id=$1`, id).Scan(&familyID); err != nil {
		return "", "", time.Time{}, time.Time{}, "", "", "", err
	}
	refreshExpires := sessionExpires
	if _, err = tx.Exec(ctx, `INSERT INTO identity_refresh_tokens(id,session_id,token_hash,token_family_id,parent_token_id,issued_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, newID, sessionID, tokenHash(newRaw), familyID, id, now, refreshExpires); err != nil {
		return "", "", time.Time{}, time.Time{}, "", "", "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE identity_refresh_tokens SET used_at=$2,replaced_by_token_id=$3 WHERE id=$1`, id, now, newID); err != nil {
		return "", "", time.Time{}, time.Time{}, "", "", "", err
	}
	accessExpires := now.Add(m.AccessTTL)
	access, err := m.sign(tokenClaims{Issuer: m.Issuer, Audience: m.Audience, Subject: operatorID, TokenID: uuid.NewString(), SessionID: sessionID.String(), OperatorID: operatorID, DeviceID: storedDevice, WarehouseID: warehouse, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: accessExpires.Unix()})
	if err != nil {
		return "", "", time.Time{}, time.Time{}, "", "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", time.Time{}, time.Time{}, "", "", "", err
	}
	return access, newRaw, accessExpires, refreshExpires, operatorID, storedDevice, warehouse, nil
}
func (m *SessionManager) Logout(ctx context.Context, token string, now time.Time) error {
	c, err := m.parse(token, now)
	if err != nil {
		return err
	}
	_, err = m.Store.Pool.Exec(ctx, `UPDATE identity_sessions SET status='REVOKED',revoked_at=$2,revocation_reason='logout' WHERE id=$1 AND status='ACTIVE'`, c.SessionID, now)
	return err
}
func (m *SessionManager) RevokeRefresh(ctx context.Context, raw string, now time.Time) error {
	tx, err := m.Store.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var sessionID uuid.UUID
	var status string
	if err := tx.QueryRow(ctx, `SELECT r.session_id,s.status FROM identity_refresh_tokens r JOIN identity_sessions s ON s.id=r.session_id WHERE r.token_hash=$1 FOR UPDATE`, tokenHash(raw)).Scan(&sessionID, &status); err != nil {
		return ports.ErrRefreshTokenInvalid
	}
	if status != "ACTIVE" {
		return ports.ErrSessionRevoked
	}
	if _, err = tx.Exec(ctx, `UPDATE identity_sessions SET status='REVOKED',revoked_at=$2,revocation_reason='logout' WHERE id=$1`, sessionID, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE identity_refresh_tokens SET revoked_at=COALESCE(revoked_at,$2) WHERE session_id=$1 AND revoked_at IS NULL`, sessionID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (m *SessionManager) sign(c tokenClaims) (string, error) {
	algorithm := "HS256"
	if m.SigningMode == "RS256" {
		algorithm = "RS256"
	}
	header, _ := json.Marshal(map[string]string{"alg": algorithm, "typ": "JWT", "kid": m.KeyID})
	h := base64.RawURLEncoding.EncodeToString(header)
	p, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(p)
	unsigned := h + "." + payload
	if m.SigningMode == "RS256" {
		digest := sha256.Sum256([]byte(unsigned))
		signature, err := rsa.SignPKCS1v15(nil, m.PrivateKey, crypto.SHA256, digest[:])
		if err != nil {
			return "", err
		}
		return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
	}
	mac := hmac.New(sha256.New, m.Secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (m *SessionManager) parse(raw string, now time.Time) (tokenClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return tokenClaims{}, ports.ErrAccessTokenInvalid
	}
	headerData, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenClaims{}, ports.ErrAccessTokenInvalid
	}
	var header struct {
		Alg   string `json:"alg"`
		KeyID string `json:"kid"`
	}
	if json.Unmarshal(headerData, &header) != nil {
		return tokenClaims{}, ports.ErrAccessTokenInvalid
	}
	unsigned := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return tokenClaims{}, ports.ErrAccessTokenInvalid
	}
	if m.SigningMode == "RS256" {
		if header.Alg != "RS256" {
			return tokenClaims{}, ports.ErrAccessTokenInvalid
		}
		publicKey := m.PublicKeys[header.KeyID]
		if publicKey == nil {
			return tokenClaims{}, ports.ErrAccessTokenInvalid
		}
		digest := sha256.Sum256([]byte(unsigned))
		if rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature) != nil {
			return tokenClaims{}, ports.ErrAccessTokenInvalid
		}
	} else {
		if header.Alg != "HS256" {
			return tokenClaims{}, ports.ErrAccessTokenInvalid
		}
		mac := hmac.New(sha256.New, m.Secret)
		_, _ = mac.Write([]byte(unsigned))
		if !hmac.Equal(mac.Sum(nil), signature) {
			return tokenClaims{}, ports.ErrAccessTokenInvalid
		}
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenClaims{}, ports.ErrAccessTokenInvalid
	}
	var c tokenClaims
	if err = json.Unmarshal(data, &c); err != nil {
		return c, ports.ErrAccessTokenInvalid
	}
	if c.Issuer != m.Issuer || c.Audience != m.Audience || c.OperatorID == "" || c.SessionID == "" || time.Unix(c.NotBefore, 0).After(now) {
		return tokenClaims{}, ports.ErrAccessTokenInvalid
	}
	if !time.Unix(c.ExpiresAt, 0).After(now) {
		return tokenClaims{}, ports.ErrAccessTokenExpired
	}
	return c, nil
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func tokenHash(raw string) []byte { h := sha256.Sum256([]byte(raw)); return h[:] }

var _ ports.SessionManager = (*SessionManager)(nil)
