package pg

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestApiKeyManager(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	apiKeyManager := e.(ApiKeyManager)

	// given
	secretId := "test-secret-id"

	// when
	apiKey, authorization, err := apiKeyManager.CreateApiKey(context.Background(), secretId)
	if err != nil {
		t.Fatalf("failed to create API key: %v", err)
	}

	// then
	assert.Equal(int32(1), apiKey.Id)
	assert.NotEmpty(apiKey.CreatedAt)
	assert.Equal(secretId, apiKey.SecretId)

	assert.NotEmpty(authorization)

	// when
	result, err := apiKeyManager.GetApiKey(context.Background(), authorization)
	if err != nil {
		t.Fatalf("failed to get API key: %v", err)
	}

	assert.Equal(apiKey, result)

	t.Run("create API key", func(t *testing.T) {
		t.Run("returns error when secret ID is empty", func(t *testing.T) {
			_, _, err := apiKeyManager.CreateApiKey(context.Background(), "")
			assert.NotNil(err)
		})

		t.Run("returns error when secret ID already exists", func(t *testing.T) {
			_, _, err := apiKeyManager.CreateApiKey(context.Background(), secretId)
			assert.NotNil(err)
			assert.Contains(err.Error(), secretId)
		})
	})

	t.Run("get API key", func(t *testing.T) {
		t.Run("returns error when authorization is empty", func(t *testing.T) {
			_, err := apiKeyManager.GetApiKey(context.Background(), "")
			assert.NotNil(err)
		})

		t.Run("returns error when authorization is invalid base64 value", func(t *testing.T) {
			_, err := apiKeyManager.GetApiKey(context.Background(), "-")
			assert.NotNil(err)
		})

		t.Run("returns error when authorization is invalid", func(t *testing.T) {
			_, err := apiKeyManager.GetApiKey(context.Background(), "c2VjcmV0SWRUZXN0")
			assert.NotNil(err)
		})

		t.Run("returns error when authorization not exists", func(t *testing.T) {
			_, err := apiKeyManager.GetApiKey(context.Background(), "c2VjcmV0SWRUZXN0Oi94ZkFRaU1PbDJPSTl2R29XUVgwd0pCTlAxbzJKcURkNldna3h3Uk12cjJN")
			assert.NotNil(err)
			assert.Equal(pgx.ErrNoRows, err)
		})
	})
}
