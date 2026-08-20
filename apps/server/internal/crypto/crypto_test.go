package crypto

import (
	"encoding/base64"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestEncryptDecrypt(t *testing.T) {
	key := testKey(t)
	secret := "sk-or-v1-supersecret-abc123"
	enc, err := Encrypt(secret, key)
	if err != nil {
		t.Fatalf("암호화 실패: %v", err)
	}
	if enc == secret {
		t.Fatal("암호화가 되지 않음")
	}
	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatalf("복호화 실패: %v", err)
	}
	if dec != secret {
		t.Errorf("복호화 결과 불일치: %q", dec)
	}
}

func TestUniqueNonce(t *testing.T) {
	key := testKey(t)
	a, _ := Encrypt("same", key)
	b, _ := Encrypt("same", key)
	if a == b {
		t.Error("동일 평문의 nonce가 같음 — 재사용 위험")
	}
}

func TestWrongKeyFails(t *testing.T) {
	keyA := testKey(t)
	raw := make([]byte, 32)
	raw[0] = 1
	keyB := base64.StdEncoding.EncodeToString(raw)

	enc, _ := Encrypt("secret", keyA)
	if _, err := Decrypt(enc, keyB); err == nil {
		t.Fatal("잘못된 키로 복호화 성공")
	}
}

func TestTamperedFails(t *testing.T) {
	key := testKey(t)
	enc, _ := Encrypt("secret", key)
	tampered := enc[:len(enc)-4] + "AAAA"
	if _, err := Decrypt(tampered, key); err == nil {
		t.Fatal("변조된 암호문 복호화 성공")
	}
}

func TestInvalidKey(t *testing.T) {
	if _, err := Encrypt("x", "short"); err == nil {
		t.Error("잘못된 키로 암호화 성공")
	}
}
