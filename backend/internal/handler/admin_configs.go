package handler

import (
	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// ConfigHandler 系统配置中心（仅超级管理员）。
type ConfigHandler struct {
	store *service.ConfigStore
	cfg   *config.Config
}

func NewConfigHandler(store *service.ConfigStore, cfg *config.Config) *ConfigHandler {
	return &ConfigHandler{store: store, cfg: cfg}
}

// List 返回全部分组配置与 .env 只读项。
func (h *ConfigHandler) List(c *gin.Context) {
	if requireSuperAdmin(c) == nil {
		return
	}
	response.OK(c, gin.H{
		"groups":    h.store.ConfigGroups(),
		"read_only": h.readOnlyItems(),
	})
}

// Save 批量保存配置（写审计，不记录值）。
func (h *ConfigHandler) Save(c *gin.Context) {
	u := requireSuperAdmin(c)
	if u == nil {
		return
	}
	var req struct {
		Values map[string]string `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	changed, err := h.store.SaveConfigs(req.Values)
	if err != nil {
		response.FailCode(c, errs.ErrValidation, err.Error())
		return
	}
	if len(changed) > 0 {
		service.Audit(h.store.DB(), u.ID, "config.update", "system_config", "",
			"更新配置 "+itoaJoin(changed))
		// 维护模式：同步 nginx 标志文件与静态维护页（与前后端是否在线无关）
		if hasAnyKey(changed, "maintenance_enabled", "maintenance_message") {
			if err := service.SyncMaintenance(h.cfg, h.store); err != nil {
				response.FailCode(c, errs.CodeInternal, "维护状态同步失败："+err.Error())
				return
			}
		}
	}
	response.OK(c, gin.H{"changed": changed})
}

// hasAnyKey 判断变更键列表中是否包含指定键。
func hasAnyKey(keys []string, targets ...string) bool {
	for _, k := range keys {
		for _, t := range targets {
			if k == t {
				return true
			}
		}
	}
	return false
}

// readOnlyItems 展示 .env 中的密钥配置状态（不回显任何内容）。
func (h *ConfigHandler) readOnlyItems() []service.ReadOnlyItem {
	status := func(ok bool, detail string) service.ReadOnlyItem {
		st := "unset"
		if ok {
			st = "configured"
		}
		return service.ReadOnlyItem{Status: st, Detail: detail}
	}
	return []service.ReadOnlyItem{
		func() service.ReadOnlyItem {
			it := status(h.cfg.CosSecretID != "" && h.cfg.CosSecretKey != "", "用于图片存储与内容审核")
			it.Key = "cos_secret"
			it.Label = "腾讯云 SecretId / SecretKey"
			return it
		}(),
		func() service.ReadOnlyItem {
			it := status(h.cfg.CosBucket != "" && h.cfg.CosRegion != "", h.cfg.CosBucket+" / "+h.cfg.CosRegion)
			it.Key = "cos_bucket"
			it.Label = "COS 存储桶"
			return it
		}(),
		func() service.ReadOnlyItem {
			it := status(h.cfg.AESKeyB64 != "", "用于加密 SMTP 密码与 AI 密钥")
			it.Key = "aes_key"
			it.Label = "配置加密密钥（AES）"
			return it
		}(),
		func() service.ReadOnlyItem {
			it := status(h.cfg.JWTSecret != "" && h.cfg.JWTSecret != "change-me-in-prod-please-use-long-random", "用于登录令牌签发")
			it.Key = "jwt_secret"
			it.Label = "JWT 密钥"
			return it
		}(),
		func() service.ReadOnlyItem {
			it := status(h.cfg.RedisConfigured, h.cfg.RedisAddr)
			it.Key = "redis"
			it.Label = "Redis"
			return it
		}(),
		func() service.ReadOnlyItem {
			it := status(h.cfg.CosCallbackToken != "", "配置后校验数据万象回调来源")
			it.Key = "cos_callback_token"
			it.Label = "审核回调校验 Token"
			return it
		}(),
	}
}

// itoaJoin 拼接变更键名（仅键名，绝不记录值）。
func itoaJoin(keys []string) string {
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
}
