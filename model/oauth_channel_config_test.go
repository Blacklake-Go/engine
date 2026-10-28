package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthSecretEncryption(t *testing.T) {
	common.CryptoSecret = "test-crypto-secret-for-oauth-channels-12345"

	rawSecret := "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21!@#"
	encrypted, err := EncryptOAuthSecret(rawSecret)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, rawSecret, encrypted)

	decrypted, err := DecryptOAuthSecret(encrypted)
	require.NoError(t, err)
	assert.Equal(t, rawSecret, decrypted)

	// Empty string case
	encEmpty, err := EncryptOAuthSecret("")
	require.NoError(t, err)
	assert.Empty(t, encEmpty)

	decEmpty, err := DecryptOAuthSecret("")
	require.NoError(t, err)
	assert.Empty(t, decEmpty)
}
