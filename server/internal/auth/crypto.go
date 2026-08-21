// Package auth 提供密钥派生、对称加密与会话签发/校验。
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// DeriveKey 由 NS_SECRET_KEY 派生 32 字节密钥。
//
// 依据 SPEC 3.4/3.5：密钥 = SHA-256(NS_SECRET_KEY)，任意长度输入均可。
// 这里直接对原始字符串做 SHA-256，因此无论输入是 base64、明文还是空串都可用。
func DeriveKey(secret string) [32]byte {
	return sha256.Sum256([]byte(secret))
}

// Encrypt 使用 AES-256-GCM 加密明文，返回 base64 编码的密文与 nonce。
func Encrypt(key [32]byte, plaintext []byte) (ciphertextB64, nonceB64 string, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), base64.StdEncoding.EncodeToString(nonce), nil
}

// Decrypt 解密由 Encrypt 生成的密文。
func Decrypt(key [32]byte, ciphertextB64, nonceB64 string) ([]byte, error) {
	if ciphertextB64 == "" || nonceB64 == "" {
		return nil, errors.New("密文或 nonce 为空")
	}
	ct, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, nil)
}
