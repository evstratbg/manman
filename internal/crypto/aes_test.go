package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestDecryptCBCPKCS7_RoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	plaintext := "secret-value"
	payload := encryptPayload(t, key, plaintext)

	got, err := DecryptCBCPKCS7(hex.EncodeToString(key), payload)
	if err != nil {
		t.Fatalf("DecryptCBCPKCS7: %v", err)
	}
	if got != plaintext {
		t.Fatalf("plaintext = %q, want %q", got, plaintext)
	}
}

func TestDecryptCBCPKCS7_Errors(t *testing.T) {
	if _, err := DecryptCBCPKCS7("zz", "00"); err == nil {
		t.Fatal("expected decode key error")
	}
	if _, err := DecryptCBCPKCS7(hex.EncodeToString([]byte("short")), "00"); err == nil {
		t.Fatal("expected invalid key length error")
	}
	if _, err := DecryptCBCPKCS7(hex.EncodeToString([]byte("0123456789abcdef")), "zz"); err == nil {
		t.Fatal("expected decode payload error")
	}
	if _, err := DecryptCBCPKCS7(hex.EncodeToString([]byte("0123456789abcdef")), "00"); err == nil {
		t.Fatal("expected ciphertext too short")
	}

	// iv_size=16, but no iv bytes in payload
	if _, err := DecryptCBCPKCS7(hex.EncodeToString([]byte("0123456789abcdef")), "1000"); err == nil {
		t.Fatal("expected ciphertext missing iv bytes")
	}

	// iv_size=16, iv present, ciphertext len=1 (not multiple of block size)
	raw := make([]byte, 2+16+1)
	binary.LittleEndian.PutUint16(raw[:2], 16)
	if _, err := DecryptCBCPKCS7(hex.EncodeToString([]byte("0123456789abcdef")), hex.EncodeToString(raw)); err == nil {
		t.Fatal("expected block size error")
	}

	key := []byte("0123456789abcdef")
	payload := encryptPayload(t, key, "secret-value")
	decoded, err := hex.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	decoded[len(decoded)-1] ^= 0x01
	if _, err := DecryptCBCPKCS7(hex.EncodeToString(key), hex.EncodeToString(decoded)); err == nil {
		t.Fatal("expected invalid padding error")
	}
}

func TestPkcs7Unpad_Errors(t *testing.T) {
	if _, err := pkcs7Unpad(nil, aes.BlockSize); err == nil {
		t.Fatal("expected invalid padded data")
	}
	if _, err := pkcs7Unpad([]byte{1, 2, 3}, aes.BlockSize); err == nil {
		t.Fatal("expected invalid padded data")
	}
	if _, err := pkcs7Unpad(bytes.Repeat([]byte{0x00}, aes.BlockSize), aes.BlockSize); err == nil {
		t.Fatal("expected invalid padding")
	}

	bad := append(bytes.Repeat([]byte{'A'}, aes.BlockSize-2), byte(0x02), byte(0x03))
	if _, err := pkcs7Unpad(bad, aes.BlockSize); err == nil {
		t.Fatal("expected invalid padding bytes")
	}
}

func encryptPayload(t *testing.T, key []byte, plaintext string) string {
	t.Helper()

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("rand iv: %v", err)
	}

	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	raw := make([]byte, 2+len(iv)+len(ciphertext))
	binary.LittleEndian.PutUint16(raw[:2], uint16(len(iv)))
	copy(raw[2:], iv)
	copy(raw[2+len(iv):], ciphertext)
	return hex.EncodeToString(raw)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - (len(data) % blockSize)
	if padLen == 0 {
		padLen = blockSize
	}
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(data, padding...)
}
