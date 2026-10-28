package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("google", &GoogleProvider{})
}

// GoogleProvider implements OAuth 2.0 / OIDC for Google
type GoogleProvider struct{}

type googleOAuthResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

func (p *GoogleProvider) GetName() string {
	return "Google"
}

func (p *GoogleProvider) IsEnabled() bool {
	return common.GoogleOAuthEnabled
}

func (p *GoogleProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(common.NewMessage(msgInvalidCode))
	}

	redirectUri := ""
	if c != nil {
		scheme := "https"
		if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		host := c.Request.Host
		if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
		}
		if system_setting.ServerAddress != "" {
			redirectUri = fmt.Sprintf("%s/oauth/google", strings.TrimRight(system_setting.ServerAddress, "/"))
		} else {
			redirectUri = fmt.Sprintf("%s://%s/oauth/google", scheme, host)
		}
	} else if system_setting.ServerAddress != "" {
		redirectUri = fmt.Sprintf("%s/oauth/google", strings.TrimRight(system_setting.ServerAddress, "/"))
	}

	values := url.Values{}
	values.Set("client_id", common.GoogleClientId)
	values.Set("client_secret", common.GoogleClientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", redirectUri)

	logger.LogDebug(ctx, "[OAuth-Google] ExchangeToken: redirect_uri=%s", redirectUri)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, common.LogText("[OAuth-Google] ExchangeToken error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(common.NewMessage(msgConnectFailed, map[string]any{"provider": "Google"}), err.Error())
	}
	defer res.Body.Close()

	logger.LogDebug(ctx, "[OAuth-Google] ExchangeToken response status: %d", res.StatusCode)

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		logger.LogError(ctx, common.LogText("[OAuth-Google] ExchangeToken failed: status=%d, body=%s", res.StatusCode, bodyStr))
		return nil, NewOAuthErrorWithRaw(common.NewMessage(msgTokenFailed, map[string]any{"provider": "Google"}), fmt.Sprintf("status %d", res.StatusCode))
	}

	var oAuthResponse googleOAuthResponse
	if err := json.NewDecoder(res.Body).Decode(&oAuthResponse); err != nil {
		logger.LogError(ctx, common.LogText("[OAuth-Google] ExchangeToken decode error: %s", err.Error()))
		return nil, err
	}

	if oAuthResponse.AccessToken == "" {
		logger.LogError(ctx, common.LogText("[OAuth-Google] ExchangeToken failed: empty access token"))
		return nil, NewOAuthError(common.NewMessage(msgTokenFailed, map[string]any{"provider": "Google"}))
	}

	return &OAuthToken{
		AccessToken:  oAuthResponse.AccessToken,
		TokenType:    oAuthResponse.TokenType,
		Scope:        oAuthResponse.Scope,
		IDToken:      oAuthResponse.IDToken,
		RefreshToken: oAuthResponse.RefreshToken,
	}, nil
}

func (p *GoogleProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	logger.LogDebug(ctx, "[OAuth-Google] GetUserInfo: fetching user info")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, common.LogText("[OAuth-Google] GetUserInfo error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(common.NewMessage(msgConnectFailed, map[string]any{"provider": "Google"}), err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		logger.LogError(ctx, common.LogText("[OAuth-Google] GetUserInfo failed: status=%d, body=%s", res.StatusCode, bodyStr))
		return nil, NewOAuthErrorWithRaw(common.NewMessage(msgGetUserFailed), fmt.Sprintf("status %d", res.StatusCode))
	}

	var info googleUserInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		logger.LogError(ctx, common.LogText("[OAuth-Google] GetUserInfo decode error: %s", err.Error()))
		return nil, err
	}

	if info.Sub == "" {
		logger.LogError(ctx, common.LogText("[OAuth-Google] GetUserInfo failed: empty sub field"))
		return nil, NewOAuthError(common.NewMessage(msgUserInfoEmpty, map[string]any{"provider": "Google"}))
	}

	username := ""
	if info.Email != "" {
		parts := strings.Split(info.Email, "@")
		username = parts[0]
	}

	return &OAuthUser{
		ProviderUserID: info.Sub,
		Username:       username,
		DisplayName:    info.Name,
		Email:          info.Email,
		Extra: map[string]any{
			"email_verified": info.EmailVerified,
			"picture":        info.Picture,
		},
	}, nil
}

// GetVerifiedEmails returns Google verified emails
func (p *GoogleProvider) GetVerifiedEmails(ctx context.Context, token *OAuthToken) ([]string, error) {
	userInfo, err := p.GetUserInfo(ctx, token)
	if err != nil {
		return nil, err
	}
	if userInfo.Email != "" {
		if verified, ok := userInfo.Extra["email_verified"].(bool); ok && verified {
			return []string{userInfo.Email}, nil
		}
	}
	return nil, nil
}

func (p *GoogleProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsGoogleIdAlreadyTaken(providerUserID)
}

func (p *GoogleProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.GoogleId = providerUserID
	return user.FillUserByGoogleId()
}

func (p *GoogleProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GoogleId = providerUserID
}

func (p *GoogleProvider) GetProviderPrefix() string {
	return "google_"
}

func (p *GoogleProvider) ProviderUserIDColumn() string {
	return "google_id"
}
