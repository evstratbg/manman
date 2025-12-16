package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

// DecryptCBCPKCS7 decrypts AES-CBC encrypted hex payload using hex encoded key.
// Payload format matches src/utils/encrypter.py: <iv_len><iv><ciphertext>, all hex encoded.
func DecryptCBCPKCS7(hexKey string, payload string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("decode key: %w", err)
	}

	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("invalid key length: %d", len(key))
	}

	raw, err := hex.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}

	if len(raw) < 2 {
		return "", errors.New("ciphertext too short")
	}

	ivSize := int(binary.LittleEndian.Uint16(raw[:2]))
	if len(raw) < 2+ivSize {
		return "", errors.New("ciphertext missing iv bytes")
	}

	iv := raw[2 : 2+ivSize]
	ciphertext := raw[2+ivSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("init cipher: %w", err)
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	unpadded, err := pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", err
	}

	return string(unpadded), nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padded data")
	}

	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(data) {
		return nil, errors.New("invalid padding")
	}

	for i := 0; i < padLen; i++ {
		if data[len(data)-1-i] != byte(padLen) {
			return nil, errors.New("invalid padding bytes")
		}
	}

	return data[:len(data)-padLen], nil
}
