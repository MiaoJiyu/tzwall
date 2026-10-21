package service

import (
	"fmt"
	"strconv"
	"strings"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/pkg/crypto"
)

// secretPrefix 密文前缀：用于区分「已保存的密钥」与明文（兼容历史明文值）。
const secretPrefix = "enc:v1:"

// ConfigGroupView 配置分组（含分组内全部条目）。
type ConfigGroupView struct {
	Key   string           `json:"key"`
	Label string           `json:"label"`
	Items []ConfigItemView `json:"items"`
}

// ConfigItemView 配置条目视图。敏感项只返回是否已设置，不回显任何明文。
type ConfigItemView struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Type    string `json:"type"`
	Desc    string `json:"desc"`
	Value   string `json:"value"`
	Default string `json:"default"`
	Min     int    `json:"min,omitempty"`
	Max     int    `json:"max,omitempty"`
	Restart bool   `json:"restart"`
	Secret  bool   `json:"secret"`  // 是否敏感项
	HasSet  bool   `json:"has_set"` // 敏感项是否已保存
	Masked  string `json:"masked"`  // 敏感项占位符
}

// ReadOnlyItem 只读展示项（来自 .env，后台不可修改）。
type ReadOnlyItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"` // configured / unset
	Detail string `json:"detail"`
}

// ConfigView 配置中心页面数据。
type ConfigView struct {
	Groups   []ConfigGroupView `json:"groups"`
	ReadOnly []ReadOnlyItem    `json:"read_only"`
}

// ValidateConfigValue 按目录校验单个配置值，返回规范化后的字符串。
func ValidateConfigValue(item config.ConfigItem, raw string) (string, error) {
	switch item.Type {
	case config.TypeBool:
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true", "1", "on", "yes":
			return "true", nil
		case "false", "0", "off", "no":
			return "false", nil
		case "":
			return item.Default, nil
		}
		return "", fmt.Errorf("%s 必须是布尔值", item.Label)
	case config.TypeInt:
		v := strings.TrimSpace(raw)
		if v == "" {
			v = item.Default
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return "", fmt.Errorf("%s 必须是整数", item.Label)
		}
		if n < item.Min || n > item.Max {
			return "", fmt.Errorf("%s 需在 %d ~ %d 之间", item.Label, item.Min, item.Max)
		}
		return strconv.Itoa(n), nil
	case config.TypeSecret:
		return strings.TrimSpace(raw), nil
	default:
		return strings.TrimSpace(raw), nil
	}
}

// ConfigGroups 按目录顺序输出分组视图。
func (s *ConfigStore) ConfigGroups() []ConfigGroupView {
	groups := make([]ConfigGroupView, 0, len(config.ConfigGroups))
	for _, g := range config.ConfigGroups {
		gv := ConfigGroupView{Key: g.Key, Label: g.Label}
		for _, it := range config.ConfigItems {
			if it.Group != g.Key {
				continue
			}
			gv.Items = append(gv.Items, s.configItemView(it))
		}
		groups = append(groups, gv)
	}
	return groups
}

func (s *ConfigStore) configItemView(it config.ConfigItem) ConfigItemView {
	v := ConfigItemView{
		Key:     it.Key,
		Label:   it.Label,
		Type:    it.Type,
		Desc:    it.Desc,
		Default: it.Default,
		Min:     it.Min,
		Max:     it.Max,
		Restart: it.Restart,
	}
	if it.Type == config.TypeSecret {
		raw := s.Get(it.Key)
		v.Secret = true
		v.HasSet = raw != ""
		v.Masked = "******"
		return v
	}
	v.Value = s.Get(it.Key)
	if v.Value == "" {
		v.Value = it.Default
	}
	return v
}

// SaveConfigs 批量保存配置：逐项校验、敏感项加密，返回变更的键列表。
func (s *ConfigStore) SaveConfigs(values map[string]string) ([]string, error) {
	changed := make([]string, 0, len(values))
	for key, raw := range values {
		item, ok := config.LookupConfig(key)
		if !ok {
			continue // 目录外键忽略，避免脏数据写入
		}
		val, err := ValidateConfigValue(item, raw)
		if err != nil {
			return nil, err
		}
		if item.Type == config.TypeSecret {
			// 空值表示不修改已保存密钥
			if val == "" {
				continue
			}
			if !strings.HasPrefix(val, secretPrefix) {
				enc, err := crypto.Encrypt(s.aesKey, val)
				if err != nil {
					return nil, err
				}
				val = secretPrefix + enc
			}
		}
		if s.Get(key) == val {
			continue
		}
		if err := s.Set(key, val); err != nil {
			return nil, err
		}
		changed = append(changed, key)
	}
	return changed, nil
}

// SecretValue 解密敏感配置；未设置或解密失败返回空串。
func (s *ConfigStore) SecretValue(key string) string {
	raw := s.Get(key)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, secretPrefix) {
		// 兼容历史明文值
		return raw
	}
	plain, err := crypto.Decrypt(s.aesKey, strings.TrimPrefix(raw, secretPrefix))
	if err != nil {
		return ""
	}
	return plain
}

// BoolValue 读取布尔配置（带默认值）。
func (s *ConfigStore) BoolValue(key string, def bool) bool {
	v := s.Get(key)
	if v == "" {
		return def
	}
	return v == "true"
}
