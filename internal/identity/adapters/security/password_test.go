package security

import "testing"

func TestArgon2idHashAndVerify(t *testing.T) {
	hasher := Argon2id{Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: 32}
	hash, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if err := hasher.Verify(hash, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := hasher.Verify(hash, "wrong password"); err == nil {
		t.Fatal("wrong password accepted")
	}
}
