package security

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

type RSAKeySet struct {
	KeyID      string
	Private    *rsa.PrivateKey
	PublicKeys map[string]*rsa.PublicKey
}

func LoadRSAKeySet(privatePath, publicPaths, keyID string) (RSAKeySet, error) {
	privateData, err := os.ReadFile(privatePath)
	if err != nil {
		return RSAKeySet{}, fmt.Errorf("read token private key: %w", err)
	}
	private, err := parsePrivateKey(privateData)
	if err != nil {
		return RSAKeySet{}, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, configured := range strings.Split(publicPaths, ",") {
		parts := strings.SplitN(strings.TrimSpace(configured), "=", 2)
		id, path := keyID, strings.TrimSpace(parts[0])
		if len(parts) == 2 {
			id, path = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return RSAKeySet{}, fmt.Errorf("read token public key: %w", err)
		}
		public, err := parsePublicKey(data)
		if err != nil {
			return RSAKeySet{}, err
		}
		keys[id] = public
	}
	if keys[keyID] == nil {
		keys[keyID] = &private.PublicKey
	}
	return RSAKeySet{KeyID: keyID, Private: private, PublicKeys: keys}, nil
}
func parsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("token private key is not PEM")
	}
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if key, ok := parsed.(*rsa.PrivateKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("token private key is not RSA")
}
func parsePublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("token public key is not PEM")
	}
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("token public key is not RSA")
}
