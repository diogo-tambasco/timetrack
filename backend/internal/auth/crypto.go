// Package auth: OAuth Google, sessão por cookie e primitivas de segurança.
// crypto.go — primitivas puras (sem HTTP, sem DB): token de sessão (D-3) e
// AES-256-GCM pros tokens do Google em repouso (D-5).
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// NewSessionToken gera o token opaco do cookie (32 bytes de crypto/rand em
// base64url) e o hash que vai pro banco — o token em claro nunca é gravado.
func NewSessionToken() (token, hash string) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic(fmt.Errorf("crypto/rand indisponível: %w", err)) // sem entropia não há sessão segura
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token)
}

// HashToken devolve o SHA-256 hex do token (lookup O(1) pela PK).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Seal cifra plain com AES-256-GCM; o blob é nonce || ciphertext (D-5).
func Seal(key, plain []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("gerar nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

// Open decifra um blob produzido por Seal, autenticando o conteúdo.
func Open(key, blob []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, fmt.Errorf("blob menor que o nonce")
	}
	nonce, ciphertext := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decifrar token: %w", err)
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("chave AES-256 exige 32 bytes, veio %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
