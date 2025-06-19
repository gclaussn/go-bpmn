package engine

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

func NewEncryption(keys string) (Encryption, error) {
	if keys == "" {
		return Encryption{}, nil
	}

	split := strings.Split(keys, ",")

	decodedKeys := make([][]byte, len(split))
	for i := range split {
		for j := i + 1; j < len(split); j++ {
			if split[i] == split[j] {
				return Encryption{}, fmt.Errorf("duplicate encryption key #%d and #%d", i, j)
			}
		}

		k, err := base64.StdEncoding.DecodeString(split[i])
		if err != nil {
			return Encryption{}, fmt.Errorf("failed to decode encryption key #%d: %v", i, err)
		}
		if len(k) != 32 {
			return Encryption{}, fmt.Errorf("failed to decode encryption key #%d: expected a length of 32, but got %d", i, len(k))
		}
		decodedKeys[i] = k
	}

	return Encryption{keys: decodedKeys}, nil
}

func NewEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to create random key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// Encryption is used to encrypt and decrypt variable data.
// The zero value is unable to perform any encryption or decryption, since no encryption keys are set.
type Encryption struct {
	keys [][]byte
}

func (e Encryption) Decrypt(encryptedValue string) (string, error) {
	if len(e.keys) == 0 {
		return "", errors.New("no encryption keys configured")
	}

	b, err := base64.StdEncoding.DecodeString(encryptedValue)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted value: %v", err)
	}

	var decryptErr error
	for _, key := range e.keys {
		c, err := aes.NewCipher([]byte(key))
		if err != nil {
			return "", fmt.Errorf("failed to create AES cipher: %v", err)
		}

		gcm, err := cipher.NewGCM(c)
		if err != nil {
			return "", fmt.Errorf("failed to create GCM: %v", err)
		}

		nonceSize := gcm.NonceSize()
		nonce, ciphertext := b[:nonceSize], b[nonceSize:]

		value, err := gcm.Open(nil, []byte(nonce), []byte(ciphertext), nil)
		if err != nil {
			decryptErr = err
			continue
		}

		return string(value), nil
	}

	return "", decryptErr
}

func (e Encryption) DecryptData(data *Data) error {
	if !data.IsEncrypted {
		return nil
	}

	value, err := e.Decrypt(data.Value)
	if err != nil {
		return err
	}

	data.Value = value
	return nil
}

func (e Encryption) Encrypt(value string) (string, error) {
	if len(e.keys) == 0 {
		return "", errors.New("no encryption keys configured")
	}

	c, err := aes.NewCipher(e.keys[0])
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to randomly generate nonce: %v", err)
	}

	b := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(b), nil
}

func (e Encryption) EncryptData(data *Data) error {
	if !data.IsEncrypted {
		return nil
	}

	encryptedValue, err := e.Encrypt(data.Value)
	if err != nil {
		return err
	}

	data.Value = encryptedValue
	return nil
}

func (e Encryption) IsZero() bool {
	return len(e.keys) == 0
}
