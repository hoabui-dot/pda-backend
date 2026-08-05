package security

import (
	"fmt"

	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"golang.org/x/crypto/argon2"
	"strings"
)

type Argon2id struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

func DefaultArgon2id() Argon2id { return Argon2id{Time: 1, Memory: 64 * 1024, Threads: 4, KeyLen: 32} }

func (a Argon2id) Hash(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	if a.Time == 0 {
		a = DefaultArgon2id()
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, a.Time, a.Memory, a.Threads, a.KeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", a.Memory, a.Time, a.Threads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

func (a Argon2id) Verify(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return fmt.Errorf("invalid password hash")
	}
	var memory, timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return fmt.Errorf("invalid password parameters")
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return err
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil {
		return err
	}
	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return fmt.Errorf("invalid password")
	}
	return nil
}
