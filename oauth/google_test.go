package oauth

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleProviderRegistration(t *testing.T) {
	provider := GetProvider("google")
	require.NotNil(t, provider)
	assert.Equal(t, "Google", provider.GetName())
	assert.Equal(t, "google_", provider.GetProviderPrefix())
	assert.Equal(t, "google_id", provider.ProviderUserIDColumn())

	user := &model.User{}
	provider.SetProviderUserID(user, "test-google-sub-12345")
	assert.Equal(t, "test-google-sub-12345", user.GoogleId)

	// ExchangeToken with empty code must fail
	_, err := provider.ExchangeToken(context.Background(), "", nil)
	assert.Error(t, err)
}
