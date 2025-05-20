package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryption(t *testing.T) {
	assert := assert.New(t)

	oldKey, err := NewEncryptionKey()
	if err != nil {
		t.Fatalf("failed to create encryption key: %v", err)
	}

	newKey, err := NewEncryptionKey()
	if err != nil {
		t.Fatalf("failed to create encryption key: %v", err)
	}

	oldEncryption, err := NewEncryption(oldKey)
	if err != nil {
		t.Fatalf("failed to create encryption: %v", err)
	}

	assert.Len(oldEncryption.keys, 1)
	assert.False(oldEncryption.IsZero())

	newEncryption, err := NewEncryption(newKey)
	if err != nil {
		t.Fatalf("failed to create encryption: %v", err)
	}

	assert.Len(newEncryption.keys, 1)
	assert.False(newEncryption.IsZero())

	encryption, err := NewEncryption(newKey + "," + oldKey)
	if err != nil {
		t.Fatalf("failed to create encryption: %v", err)
	}

	assert.Len(encryption.keys, 2)
	assert.False(encryption.IsZero())

	emptyEncryption, err := NewEncryption("")
	if err != nil {
		t.Fatalf("failed to create encryption: %v", err)
	}

	assert.True(emptyEncryption.IsZero())

	t.Run("new encryption returns error when duplicate encryption key configured", func(t *testing.T) {
		_, err := NewEncryption(newKey + "," + newKey)
		assert.NotNil(err)
	})

	t.Run("encrypt and decrypt", func(t *testing.T) {
		encryptedValue, err := encryption.Encrypt("test")
		if err != nil {
			t.Fatalf("failed to encrypt value: %v", err)
		}

		value, err := encryption.Decrypt(encryptedValue)
		if err != nil {
			t.Fatalf("failed to decrypt value: %v", err)
		}

		assert.Equal("test", value)
	})

	t.Run("encrypt with old and decrypt", func(t *testing.T) {
		encryptedValue, err := oldEncryption.Encrypt("test")
		if err != nil {
			t.Fatalf("failed to encrypt value: %v", err)
		}

		value, err := encryption.Decrypt(encryptedValue)
		if err != nil {
			t.Fatalf("failed to decrypt value: %v", err)
		}

		assert.Equal("test", value)
	})

	t.Run("encrypt with old and decrypt with new", func(t *testing.T) {
		encryptedValue, err := oldEncryption.Encrypt("test")
		if err != nil {
			t.Fatalf("failed to encrypt value: %v", err)
		}

		_, err = newEncryption.Decrypt(encryptedValue)
		assert.Error(err)
	})

	t.Run("encrypt and decrypt data marked as encrpyted", func(t *testing.T) {
		// given
		data := Data{IsEncrypted: true, Value: "test"}

		// when
		if err := encryption.EncryptData(&data); err != nil {
			t.Fatalf("failed to encrypt value: %v", err)
		}

		// then
		assert.NotEqual("test", data.Value)

		// when
		if err := encryption.DecryptData(&data); err != nil {
			t.Fatalf("failed to decrypt value: %v", err)
		}

		// then
		assert.Equal("test", data.Value)
	})

	t.Run("encrypt and decrypt data", func(t *testing.T) {
		// given
		data := Data{IsEncrypted: false, Value: "test"}

		// when
		if err := encryption.EncryptData(&data); err != nil {
			t.Fatalf("failed to encrypt value: %v", err)
		}

		// then
		assert.Equal("test", data.Value)

		// when
		if err := encryption.DecryptData(&data); err != nil {
			t.Fatalf("failed to decrypt value: %v", err)
		}

		// then
		assert.Equal("test", data.Value)
	})

	t.Run("encrypt returns error when keys are empty", func(t *testing.T) {
		_, err := emptyEncryption.Encrypt("test")
		assert.NotNil(err)
	})

	t.Run("decrypt returns error when keys are empty", func(t *testing.T) {
		_, err := emptyEncryption.Decrypt("test")
		assert.NotNil(err)
	})
}
