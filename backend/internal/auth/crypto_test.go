package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// CA-08 (parcial): token de sessão com 32 bytes de entropia; banco só vê o hash.
func TestCryptoSessao(t *testing.T) {
	token1, hash1 := NewSessionToken()
	token2, hash2 := NewSessionToken()

	if token1 == token2 {
		t.Error("dois tokens consecutivos não podem ser iguais")
	}

	raw, err := base64.RawURLEncoding.DecodeString(token1)
	if err != nil {
		t.Fatalf("token deve ser base64url: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("token deve ter 32 bytes de entropia, veio %d", len(raw))
	}

	want := sha256.Sum256([]byte(token1))
	if hash1 != hex.EncodeToString(want[:]) {
		t.Error("hash deve ser SHA-256 hex do token")
	}
	if HashToken(token1) != hash1 {
		t.Error("HashToken deve bater com o hash devolvido na criação")
	}
	if hash1 == hash2 {
		t.Error("hashes de tokens diferentes não podem colidir")
	}
}

// CA-14 (parcial): AES-256-GCM — round-trip, ciphertext ≠ plaintext,
// blob adulterado → erro.
func TestCryptoAESGCM(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	plain := []byte("ya29.token-do-google-bem-secreto")

	blob, err := Seal(key, plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(blob, plain) {
		t.Error("ciphertext não pode conter o plaintext")
	}

	got, err := Open(key, blob)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round-trip falhou: %q ≠ %q", got, plain)
	}

	// adultera 1 byte do ciphertext → autenticação GCM falha
	tampered := append([]byte(nil), blob...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := Open(key, tampered); err == nil {
		t.Error("blob adulterado deveria falhar na autenticação")
	}

	// chave de tamanho errado → erro
	if _, err := Seal([]byte("curta"), plain); err == nil {
		t.Error("chave != 32 bytes deveria falhar")
	}
}
