package config

import (
	"encoding/base64"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config 聚合所有运行时配置。敏感密钥仅来自 .env，绝不回显明文。
type Config struct {
	// ServerHost 监听地址，默认空表示所有网卡；配合 Nginx 反代时建议 127.0.0.1
	ServerHost string
	ServerPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	JWTSecret string
	AESKeyB64 string
	AESKey    []byte

	CosSecretID  string
	CosSecretKey string
	CosBucket    string
	CosRegion    string
	CosBaseURL   string
	// CosAuditBizType 数据万象审核策略模板（留空使用默认策略）
	CosAuditBizType string
	// CosCallbackToken 审核回调校验 Token（留空则不校验）
	CosCallbackToken string

	SmtpHost string
	SmtpPort int
	SmtpUser string
	SmtpPass string
	SmtpFrom string

	WatermarkFontPath string
	SiteName          string
	TrustedProxies    []string

	// 种子超管（仅在首次启动、库为空时写入）
	AdminUsername string
	AdminPassword string
	AdminEmail    string
	AdminRealName string
	AdminSchool   string

	// 派生能力标记：决定是否启用外部依赖，否则降级
	StorageConfigured    bool
	ModerationConfigured bool
	MailerConfigured     bool
	RedisConfigured      bool
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func Load() *Config {
	_ = godotenv.Load()

	c := &Config{}
	c.ServerHost = getenv("SERVER_HOST", "")
	c.ServerPort = getenv("SERVER_PORT", "80")

	c.DBHost = getenv("DB_HOST", "127.0.0.1")
	c.DBPort = getenv("DB_PORT", "5432")
	c.DBUser = getenv("DB_USER", "tzwall")
	c.DBPassword = getenv("DB_PASSWORD", "")
	c.DBName = getenv("DB_NAME", "tzwall")
	c.DBSSLMode = getenv("DB_SSLMODE", "disable")

	c.RedisAddr = getenv("REDIS_ADDR", "")
	c.RedisPassword = getenv("REDIS_PASSWORD", "")
	c.RedisDB, _ = strconv.Atoi(getenv("REDIS_DB", "0"))

	c.JWTSecret = getenv("JWT_SECRET", "change-me-in-prod-please-use-long-random")
	c.AESKeyB64 = getenv("AES_KEY_B64", "")

	defaultAES := []byte("0123456789abcdef0123456789abcdef") // 仅开发缺省
	if c.AESKeyB64 == "" {
		c.AESKey = defaultAES
	} else {
		if b, err := base64.StdEncoding.DecodeString(c.AESKeyB64); err == nil && len(b) == 32 {
			c.AESKey = b
		} else {
			c.AESKey = defaultAES
		}
	}

	c.CosSecretID = getenv("COS_SECRET_ID", "")
	c.CosSecretKey = getenv("COS_SECRET_KEY", "")
	c.CosBucket = getenv("COS_BUCKET", "")
	c.CosRegion = getenv("COS_REGION", "")
	c.CosBaseURL = getenv("COS_BASE_URL", "")
	c.CosAuditBizType = getenv("COS_AUDIT_BIZ_TYPE", "")
	c.CosCallbackToken = getenv("COS_CALLBACK_TOKEN", "")

	c.SmtpHost = getenv("SMTP_HOST", "")
	c.SmtpPort, _ = strconv.Atoi(getenv("SMTP_PORT", "465"))
	c.SmtpUser = getenv("SMTP_USER", "")
	c.SmtpPass = getenv("SMTP_PASS", "")
	c.SmtpFrom = getenv("SMTP_FROM", "")

	c.WatermarkFontPath = getenv("WATERMARK_FONT_PATH", "")
	c.SiteName = getenv("SITE_NAME", "泰兴中学校园墙")
	c.TrustedProxies = strings.Split(getenv("TRUSTED_PROXIES", "127.0.0.1"), ",")

	c.AdminUsername = getenv("ADMIN_USERNAME", "admin")
	c.AdminPassword = getenv("ADMIN_PASSWORD", "Admin@123456")
	c.AdminEmail = getenv("ADMIN_EMAIL", "admin@tzwall.local")
	c.AdminRealName = getenv("ADMIN_REAL_NAME", "系统管理员")
	c.AdminSchool = getenv("ADMIN_SCHOOL", "泰兴中学")

	c.StorageConfigured = c.CosSecretID != "" && c.CosSecretKey != "" && c.CosBucket != "" && c.CosRegion != ""
	c.ModerationConfigured = c.StorageConfigured
	c.MailerConfigured = c.SmtpHost != "" && c.SmtpUser != "" && c.SmtpPass != ""
	c.RedisConfigured = c.RedisAddr != ""

	return c
}

// DSN 返回 PostgreSQL 连接串。
func (c *Config) DSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode +
		" timezone=Asia/Shanghai"
}
