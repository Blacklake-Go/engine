package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type UpdateOAuthChannelRequest struct {
	Enabled      *bool  `json:"enabled"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scopes       string `json:"scopes"`
}

// GetOAuthChannels returns all OAuth channels with probe status and masked credentials
func GetOAuthChannels(c *gin.Context) {
	channels, err := model.GetAllOAuthChannelConfigs()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, channels)
}

// UpdateOAuthChannel updates configuration and/or secret for a specific OAuth channel
func UpdateOAuthChannel(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.ApiErrorT(c, "Invalid channel ID")
		return
	}

	var req UpdateOAuthChannelRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	cfg, err := model.GetOAuthChannelConfigById(id)
	if err != nil {
		common.ApiErrorT(c, "Channel not found")
		return
	}

	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.ClientId != "" {
		cfg.ClientId = strings.TrimSpace(req.ClientId)
	}
	if req.Scopes != "" {
		cfg.Scopes = strings.TrimSpace(req.Scopes)
	}

	secret := strings.TrimSpace(req.ClientSecret)
	if err := model.SaveOAuthChannelConfig(cfg, secret); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccessT(c, "Channel updated successfully", cfg)
}

// ProbeOAuthChannel performs an on-demand latency & TLS probe to the channel's authorization endpoint
func ProbeOAuthChannel(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.ApiErrorT(c, "Invalid channel ID")
		return
	}

	cfg, err := model.GetOAuthChannelConfigById(id)
	if err != nil {
		common.ApiErrorT(c, "Channel not found")
		return
	}

	latency, status, err := model.ProbeChannelConnectivity(cfg)
	if err != nil {
		common.ApiSuccess(c, gin.H{
			"id":               cfg.Id,
			"probe_latency_ms": -1,
			"probe_status":     "error",
			"error":            err.Error(),
		})
		return
	}

	common.ApiSuccess(c, gin.H{
		"id":               cfg.Id,
		"probe_latency_ms": latency,
		"probe_status":     status,
	})
}
