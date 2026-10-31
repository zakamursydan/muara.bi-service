package cryptohelper

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type CryptoHelper struct {
	key []byte
}

func NewCryptoHelper(masterKey string) (*CryptoHelper, error) {
	key := []byte(masterKey)
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be exactly 32 bytes for AES-256 (current: %d bytes)", len(key))
	}
	return &CryptoHelper{key: key}, nil
}

func (c *CryptoHelper) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Ciphertext has nonce prepended: [nonce (12 bytes)][ciphertext + tag]
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *CryptoHelper) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, actualCiphertext, nil)
}

func (c *CryptoHelper) EncryptJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return c.Encrypt(data)
}

func (c *CryptoHelper) DecryptJSON(ciphertext []byte, target any) error {
	plaintext, err := c.Decrypt(ciphertext)
	if err != nil {
		return err
	}
	return json.Unmarshal(plaintext, target)
}
