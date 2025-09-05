package pg

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
)

type ApiKeyManager interface {
	CreateApiKey(ctx context.Context, secretId string) (ApiKey, string, error)
	GetApiKey(ctx context.Context, authorization string) (ApiKey, error)
}

type ApiKey struct {
	Id int32

	CreatedAt time.Time
	SecretId  string
}

// apiKeyEntity is used as internal representation of an API key.
type apiKeyEntity struct {
	id int32

	createdAt  time.Time
	secretId   string
	secretHash string
}

func (e apiKeyEntity) apiKey() ApiKey {
	return ApiKey{
		Id: e.id,

		CreatedAt: e.createdAt,
		SecretId:  e.secretId,
	}
}

func createApiKey(ctx *pgContext, secretId string) (ApiKey, string, error) {
	if secretId == "" {
		return ApiKey{}, "", errors.New("secret ID is empty")
	}

	encryptionKey, err := engine.NewEncryptionKey()
	if err != nil {
		return ApiKey{}, "", fmt.Errorf("failed to create encryption key: %v", err)
	}

	encryption, _ := engine.NewEncryption(encryptionKey)

	secret, err := encryption.Encrypt(ctx.Time().Format(time.RFC3339Nano) + secretId)
	if err != nil {
		return ApiKey{}, "", fmt.Errorf("failed to create secret: %v", err)
	}

	// cut by half to reduce overall authorization length
	secret = secret[22 : len(secret)-22]

	hash := sha256.New()
	hash.Write([]byte(secret))
	secretHash := hash.Sum(nil)

	entity := apiKeyEntity{
		createdAt:  ctx.Time(),
		secretId:   secretId,
		secretHash: base64.StdEncoding.EncodeToString(secretHash),
	}

	row := ctx.tx.QueryRow(ctx.txCtx, `
INSERT INTO api_key (
	created_at,
	secret_hash,
	secret_id
) VALUES (
	$1,
	$2,
	$3
) ON CONFLICT DO NOTHING RETURNING id
`,
		entity.createdAt,
		entity.secretHash,
		entity.secretId,
	)

	if err := row.Scan(&entity.id); err != nil {
		if err == pgx.ErrNoRows {
			return ApiKey{}, "", fmt.Errorf("secret ID %s already exists", secretId)
		} else {
			return ApiKey{}, "", fmt.Errorf("failed to insert API key: %v", err)
		}
	}

	secretIdAndSecret := secretId + ":" + secret
	authorization := base64.StdEncoding.EncodeToString([]byte(secretIdAndSecret))

	return entity.apiKey(), authorization, nil
}

func getApiKey(ctx *pgContext, authorization string) (ApiKey, error) {
	if authorization == "" {
		return ApiKey{}, errors.New("authorization is empty")
	}

	b, err := base64.StdEncoding.DecodeString(authorization)
	if err != nil {
		return ApiKey{}, fmt.Errorf("failed to decode authorization: %v", err)
	}

	secretIdAndSecret := string(b)
	i := strings.LastIndex(secretIdAndSecret, ":")
	if i == -1 {
		return ApiKey{}, errors.New("failed to decode authorization: invalid format")
	}

	secretId := secretIdAndSecret[:i]
	secret := secretIdAndSecret[i+1:]

	hash := sha256.New()
	hash.Write([]byte(secret))
	secretHash := hash.Sum(nil)

	row := ctx.tx.QueryRow(ctx.txCtx, `
SELECT
	id,

	created_at
FROM
	api_key
WHERE
	secret_id = $1 AND
	secret_hash = $2
`,
		secretId,
		base64.StdEncoding.EncodeToString(secretHash),
	)

	var apiKey apiKeyEntity

	if err := row.Scan(
		&apiKey.id,

		&apiKey.createdAt,
	); err != nil {
		return ApiKey{}, err
	}

	apiKey.secretId = secretId

	return apiKey.apiKey(), nil
}
