package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// OAuthChannelConfig stores credentials, connectivity status and display settings for an OAuth provider
type OAuthChannelConfig struct {
	Id              string     `json:"id" gorm:"primaryKey;type:varchar(64)"`             // e.g. "google", "github"
	Name            string     `json:"name" gorm:"type:varchar(128);not null"`            // e.g. "Google", "GitHub"
	Category        string     `json:"category" gorm:"type:varchar(32);default:'global'"` // "mainland" or "global"
	Enabled         bool       `json:"enabled" gorm:"default:false"`                      // whether shown on login
	ClientId        string     `json:"client_id" gorm:"type:varchar(256);default:''"`
	EncryptedSecret string     `json:"-" gorm:"type:text"` // AES-256-GCM encrypted
	HasSecret       bool       `json:"has_secret" gorm:"-:all"`
	RedirectUri     string     `json:"redirect_uri" gorm:"type:varchar(512);default:''"`
	Scopes          string     `json:"scopes" gorm:"type:varchar(256);default:''"`
	ProbeLatencyMs  int        `json:"probe_latency_ms" gorm:"default:-1"`
	ProbeStatus     string     `json:"probe_status" gorm:"type:varchar(32);default:'pending'"` // 'ready', 'pending', 'error'
	LastProbedAt    *time.Time `json:"last_probed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (OAuthChannelConfig) TableName() string {
	return "oauth_channel_configs"
}

// EncryptOAuthSecret encrypts plaintext using AES-256-GCM with a key derived from CryptoSecret
func EncryptOAuthSecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	secret := common.CryptoSecret
	if secret == "" {
		secret = common.SessionSecret
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptOAuthSecret decrypts an AES-256-GCM encrypted Base64 string
func DecryptOAuthSecret(ciphertextBase64 string) (string, error) {
	if ciphertextBase64 == "" {
		return "", nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", err
	}
	secret := common.CryptoSecret
	if secret == "" {
		secret = common.SessionSecret
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// GetAllOAuthChannelConfigs returns all channel configurations
func GetAllOAuthChannelConfigs() ([]*OAuthChannelConfig, error) {
	var channels []*OAuthChannelConfig
	err := DB.Order("category desc, id asc").Find(&channels).Error
	if err != nil {
		return nil, err
	}
	for _, ch := range channels {
		ch.HasSecret = ch.EncryptedSecret != ""
		// Sync with runtime enabled state
		switch ch.Id {
		case "google":
			ch.Enabled = common.GoogleOAuthEnabled
			ch.ClientId = common.GoogleClientId
			ch.HasSecret = common.GoogleClientSecret != "" || ch.EncryptedSecret != ""
		case "github":
			ch.Enabled = common.GitHubOAuthEnabled
			ch.ClientId = common.GitHubClientId
			ch.HasSecret = common.GitHubClientSecret != "" || ch.EncryptedSecret != ""
		case "linuxdo":
			ch.Enabled = common.LinuxDOOAuthEnabled
			ch.ClientId = common.LinuxDOClientId
			ch.HasSecret = common.LinuxDOClientSecret != "" || ch.EncryptedSecret != ""
		case "wechat":
			ch.Enabled = common.WeChatAuthEnabled
			ch.HasSecret = common.WeChatServerToken != "" || ch.EncryptedSecret != ""
		case "telegram":
			ch.Enabled = common.TelegramOAuthEnabled
			ch.HasSecret = common.TelegramBotToken != "" || ch.EncryptedSecret != ""
		}
	}
	return channels, nil
}

// GetOAuthChannelConfigById returns a single channel config by its ID
func GetOAuthChannelConfigById(id string) (*OAuthChannelConfig, error) {
	var ch OAuthChannelConfig
	err := DB.Where("id = ?", id).First(&ch).Error
	if err != nil {
		return nil, err
	}
	ch.HasSecret = ch.EncryptedSecret != ""
	return &ch, nil
}

// SaveOAuthChannelConfig saves a channel config, optionally updating its secret, and syncs runtime
func SaveOAuthChannelConfig(cfg *OAuthChannelConfig, plaintextSecret string) error {
	if plaintextSecret != "" {
		encrypted, err := EncryptOAuthSecret(plaintextSecret)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret: %w", err)
		}
		cfg.EncryptedSecret = encrypted
	}

	if err := DB.Save(cfg).Error; err != nil {
		return err
	}

	// Hot sync with runtime
	SyncOAuthChannelToRuntime(cfg, plaintextSecret)
	return nil
}

// SyncOAuthChannelToRuntime applies channel config to memory options and common variables
func SyncOAuthChannelToRuntime(cfg *OAuthChannelConfig, plaintextSecret string) {
	// If plaintextSecret is empty, attempt to decrypt existing secret
	secret := plaintextSecret
	if secret == "" && cfg.EncryptedSecret != "" {
		decrypted, err := DecryptOAuthSecret(cfg.EncryptedSecret)
		if err == nil {
			secret = decrypted
		}
	}

	switch cfg.Id {
	case "google":
		common.GoogleOAuthEnabled = cfg.Enabled
		common.GoogleClientId = cfg.ClientId
		if secret != "" {
			common.GoogleClientSecret = secret
		}
		_ = UpdateOption("GoogleOAuthEnabled", strconv.FormatBool(cfg.Enabled))
		_ = UpdateOption("GoogleClientId", cfg.ClientId)
		if secret != "" {
			_ = UpdateOption("GoogleClientSecret", secret)
		}
	case "github":
		common.GitHubOAuthEnabled = cfg.Enabled
		common.GitHubClientId = cfg.ClientId
		if secret != "" {
			common.GitHubClientSecret = secret
		}
		_ = UpdateOption("GitHubOAuthEnabled", strconv.FormatBool(cfg.Enabled))
		_ = UpdateOption("GitHubClientId", cfg.ClientId)
		if secret != "" {
			_ = UpdateOption("GitHubClientSecret", secret)
		}
	case "linuxdo":
		common.LinuxDOOAuthEnabled = cfg.Enabled
		common.LinuxDOClientId = cfg.ClientId
		if secret != "" {
			common.LinuxDOClientSecret = secret
		}
		_ = UpdateOption("LinuxDOOAuthEnabled", strconv.FormatBool(cfg.Enabled))
		_ = UpdateOption("LinuxDOClientId", cfg.ClientId)
		if secret != "" {
			_ = UpdateOption("LinuxDOClientSecret", secret)
		}
	case "wechat":
		common.WeChatAuthEnabled = cfg.Enabled
		_ = UpdateOption("WeChatAuthEnabled", strconv.FormatBool(cfg.Enabled))
	case "telegram":
		common.TelegramOAuthEnabled = cfg.Enabled
		_ = UpdateOption("TelegramOAuthEnabled", strconv.FormatBool(cfg.Enabled))
	}
}

// ProbeChannelConnectivity tests connection latency to the provider's service
func ProbeChannelConnectivity(cfg *OAuthChannelConfig) (int, string, error) {
	var probeUrl string
	switch cfg.Id {
	case "google":
		probeUrl = "https://accounts.google.com"
	case "github":
		probeUrl = "https://github.com"
	case "discord":
		probeUrl = "https://discord.com"
	case "telegram":
		probeUrl = "https://api.telegram.org"
	case "linuxdo":
		probeUrl = "https://connect.linux.do"
	case "wechat":
		probeUrl = "https://api.weixin.qq.com"
	default:
		probeUrl = "https://accounts.google.com"
	}

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	start := time.Now()
	resp, err := client.Head(probeUrl)
	if err != nil {
		// Try GET fallback
		resp, err = client.Get(probeUrl)
	}

	now := time.Now()
	cfg.LastProbedAt = &now

	if err != nil {
		cfg.ProbeLatencyMs = -1
		cfg.ProbeStatus = "error"
		_ = DB.Model(cfg).Updates(map[string]any{
			"probe_latency_ms": -1,
			"probe_status":     "error",
			"last_probed_at":   now,
		})
		return -1, "error", err
	}
	defer resp.Body.Close()

	latency := int(time.Since(start).Milliseconds())
	status := "ready"
	cfg.ProbeLatencyMs = latency
	cfg.ProbeStatus = status

	_ = DB.Model(cfg).Updates(map[string]any{
		"probe_latency_ms": latency,
		"probe_status":     status,
		"last_probed_at":   now,
	})

	return latency, status, nil
}

// InitOAuthChannelConfigs seeds default OAuth channels
func InitOAuthChannelConfigs(db *gorm.DB) error {
	defaultChannels := []OAuthChannelConfig{
		{
			Id:             "google",
			Name:           "Google",
			Category:       "global",
			Enabled:        common.GoogleOAuthEnabled,
			ClientId:       common.GoogleClientId,
			Scopes:         "openid profile email",
			ProbeStatus:    "pending",
			ProbeLatencyMs: -1,
		},
		{
			Id:             "github",
			Name:           "GitHub",
			Category:       "global",
			Enabled:        common.GitHubOAuthEnabled,
			ClientId:       common.GitHubClientId,
			Scopes:         "read:user user:email",
			ProbeStatus:    "pending",
			ProbeLatencyMs: -1,
		},
		{
			Id:             "discord",
			Name:           "Discord",
			Category:       "global",
			Enabled:        false,
			Scopes:         "identify email",
			ProbeStatus:    "pending",
			ProbeLatencyMs: -1,
		},
		{
			Id:             "telegram",
			Name:           "Telegram",
			Category:       "global",
			Enabled:        common.TelegramOAuthEnabled,
			Scopes:         "",
			ProbeStatus:    "pending",
			ProbeLatencyMs: -1,
		},
		{
			Id:             "linuxdo",
			Name:           "LINUX DO",
			Category:       "mainland",
			Enabled:        common.LinuxDOOAuthEnabled,
			ClientId:       common.LinuxDOClientId,
			Scopes:         "",
			ProbeStatus:    "pending",
			ProbeLatencyMs: -1,
		},
		{
			Id:             "wechat",
			Name:           "微信 (WeChat)",
			Category:       "mainland",
			Enabled:        common.WeChatAuthEnabled,
			Scopes:         "snsapi_login",
			ProbeStatus:    "pending",
			ProbeLatencyMs: -1,
		},
		{
			Id:             "feishu",
			Name:           "飞书 (Lark)",
			Category:       "mainland",
			Enabled:        false,
			Scopes:         "contact:user.id:readonly",
			ProbeStatus:    "pending",
			ProbeLatencyMs: -1,
		},
		{
			Id:             "dingtalk",
			Name:           "钉钉 (DingTalk)",
			Category:       "mainland",
			Enabled:        false,
			Scopes:         "openid corpid",
			ProbeStatus:    "pending",
			ProbeLatencyMs: -1,
		},
	}

	for _, ch := range defaultChannels {
		var count int64
		db.Model(&OAuthChannelConfig{}).Where("id = ?", ch.Id).Count(&count)
		if count == 0 {
			// Encrypt secret if available from common
			var secretToSave string
			if ch.Id == "github" && common.GitHubClientSecret != "" {
				secretToSave = common.GitHubClientSecret
			} else if ch.Id == "google" && common.GoogleClientSecret != "" {
				secretToSave = common.GoogleClientSecret
			} else if ch.Id == "linuxdo" && common.LinuxDOClientSecret != "" {
				secretToSave = common.LinuxDOClientSecret
			}
			if secretToSave != "" {
				ch.EncryptedSecret, _ = EncryptOAuthSecret(secretToSave)
			}
			db.Create(&ch)
		}
	}
	return nil
}
