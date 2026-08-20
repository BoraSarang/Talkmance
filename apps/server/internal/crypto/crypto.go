// Package crypto — BYOK 키 AES-GCM 암/복호화
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Encrypt AES-256-GCM 암호화 → base64(nonce+ct)
// encKeyBase64: 32바이트 키의 base64 (ENV: ENC_KEY)
func Encrypt(plaintext string, encKeyBase64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(encKeyBase64)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("crypto: ENC_KEY는 32바이트 base64여야 합니다")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: AES 초기화 실패: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: GCM 초기화 실패: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce 생성 실패: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt Encrypt의 역
func Decrypt(encoded string, encKeyBase64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(encKeyBase64)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("crypto: ENC_KEY는 32바이트 base64여야 합니다")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: AES 초기화 실패: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: GCM 초기화 실패: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: base64 디코딩 실패: %w", err)
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("crypto: 암호문이 너무 짧습니다")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: 복호화 실패: %w", err)
	}
	return string(pt), nil
}
