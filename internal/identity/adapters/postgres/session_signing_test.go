package postgres

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
)

func TestRS256SessionTokensSupportKeyOverlapAndRejectUnknownKeys(t *testing.T) {
	oldKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	manager, err := NewRSASessionManager(&Store{}, map[string]*rsa.PublicKey{
		"old": &oldKey.PublicKey,
		"new": &newKey.PublicKey,
	}, newKey, "new", "pda", "pda-api", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	token, err := manager.sign(tokenClaims{
		Issuer:     manager.Issuer,
		Audience:   manager.Audience,
		OperatorID: "OP-RSA",
		SessionID:  "SESSION-RSA",
		IssuedAt:   now.Unix(),
		NotBefore:  now.Unix(),
		ExpiresAt:  now.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.parse(token, now)
	if err != nil {
		t.Fatal(err)
	}
	if claims.OperatorID != "OP-RSA" {
		t.Fatalf("operator=%q, want OP-RSA", claims.OperatorID)
	}

	oldManager, err := NewRSASessionManager(&Store{}, map[string]*rsa.PublicKey{"old": &oldKey.PublicKey}, oldKey, "old", "pda", "pda-api", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	oldToken, err := oldManager.sign(tokenClaims{
		Issuer:     oldManager.Issuer,
		Audience:   oldManager.Audience,
		OperatorID: "OP-RSA",
		SessionID:  "SESSION-RSA",
		IssuedAt:   now.Unix(),
		NotBefore:  now.Unix(),
		ExpiresAt:  now.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.parse(oldToken, now); err != nil {
		t.Fatalf("retained old key was rejected: %v", err)
	}

	unknownKeyManager, err := NewRSASessionManager(&Store{}, map[string]*rsa.PublicKey{"new": &newKey.PublicKey}, newKey, "new", "pda", "pda-api", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = unknownKeyManager.parse(oldToken, now); err == nil {
		t.Fatal("token signed by an unknown key was accepted")
	}
}
