package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrKeyTooShort       = errors.New("encryption key must be at least 32 bytes")
)

// Crypto 加密解密工具
type Crypto struct {
	key []byte
}

// NewCrypto 创建加密工具
func NewCrypto(key string) (*Crypto, error) {
	// 确保密钥长度为32字节 (AES-256)
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		// 填充密钥
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	return &Crypto{key: keyBytes}, nil
}

// Encrypt 加密字符串
func (c *Crypto) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密字符串
func (c *Crypto) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GenerateKey 生成随机密钥
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// EncryptPassword 加密密码的便捷函数
func EncryptPassword(password, key string) (string, error) {
	crypto, err := NewCrypto(key)
	if err != nil {
		return "", err
	}
	return crypto.Encrypt(password)
}

// DecryptPassword 解密密码的便捷函数
func DecryptPassword(encrypted, key string) (string, error) {
	crypto, err := NewCrypto(key)
	if err != nil {
		return "", err
	}
	return crypto.Decrypt(encrypted)
}
