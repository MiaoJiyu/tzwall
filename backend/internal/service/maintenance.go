package service

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tzwall/backend/internal/config"
)

// 维护模式由 nginx 依据「标志文件」直接返回静态维护页实现：
// 维护页存放在前端 dist 之外，因此即使后端或前端正在更新/下线，访客仍能看到维护提示。
// 后端只在管理员保存配置时同步标志文件与维护页内容。

// MaintenanceEnabled 标志文件是否存在（nginx 的判定依据）。
func MaintenanceEnabled(flagPath string) bool {
	if strings.TrimSpace(flagPath) == "" {
		return false
	}
	_, err := os.Stat(flagPath)
	return err == nil
}

// EnableMaintenance 开启维护：生成静态维护页并写入标志文件。
func EnableMaintenance(flagPath, htmlPath, message string) error {
	if strings.TrimSpace(flagPath) == "" || strings.TrimSpace(htmlPath) == "" {
		return fmt.Errorf("维护页路径未配置")
	}
	if err := os.MkdirAll(filepath.Dir(flagPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(renderMaintenanceHTML(message)), 0o644); err != nil {
		return err
	}
	return os.WriteFile(flagPath, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
}

// DisableMaintenance 关闭维护：删除标志文件（维护页文件保留，便于下次直接启用）。
func DisableMaintenance(flagPath string) error {
	if err := os.Remove(flagPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SyncMaintenance 按当前配置同步维护状态（管理员保存配置后调用）。
func SyncMaintenance(cfg *config.Config, store *ConfigStore) error {
	flag, htmlPath := maintenancePaths(cfg)
	if store.BoolValue("maintenance_enabled", false) {
		return EnableMaintenance(flag, htmlPath, store.Get("maintenance_message"))
	}
	return DisableMaintenance(flag)
}

// maintenancePaths 解析标志文件与维护页路径（默认 /opt/tzwall/run 下，可由 .env 覆盖）。
func maintenancePaths(cfg *config.Config) (string, string) {
	flag, htmlPath := "/opt/tzwall/run/MAINTENANCE", "/opt/tzwall/run/maintenance.html"
	if cfg != nil {
		if strings.TrimSpace(cfg.MaintenanceFlag) != "" {
			flag = cfg.MaintenanceFlag
		}
		if strings.TrimSpace(cfg.MaintenanceHTML) != "" {
			htmlPath = cfg.MaintenanceHTML
		}
	}
	return flag, htmlPath
}

// renderMaintenanceHTML 生成自包含维护页：无外部依赖、禁缓存、定时重试（维护结束后自动恢复）。
func renderMaintenanceHTML(message string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "站点正在更新维护，请稍后再来。"
	}
	esc := html.EscapeString(msg)
	esc = strings.ReplaceAll(esc, "\n", "<br>")
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<meta name="robots" content="noindex" />
<title>系统维护中</title>
<style>
  * { box-sizing: border-box; }
  body { margin:0; min-height:100vh; display:flex; align-items:center; justify-content:center; padding:24px;
         font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;
         background:linear-gradient(135deg,#6d28d9 0%%,#4c1d95 100%%); color:#fff; }
  .card { width:100%%; max-width:440px; background:rgba(255,255,255,.08); border:1px solid rgba(255,255,255,.18);
          border-radius:20px; padding:36px 28px; text-align:center; }
  .icon { width:64px; height:64px; margin:0 auto 18px; border-radius:50%%; background:rgba(255,255,255,.16);
          display:flex; align-items:center; justify-content:center; font-size:30px; }
  h1 { margin:0 0 12px; font-size:22px; font-weight:700; letter-spacing:.5px; }
  p  { margin:0; font-size:15px; line-height:1.7; color:rgba(255,255,255,.88); }
  .tip{ margin-top:20px; font-size:12px; color:rgba(255,255,255,.6); }
  .bar{ margin-top:22px; height:4px; border-radius:999px; background:rgba(255,255,255,.18); overflow:hidden; }
  .bar i{ display:block; height:100%%; width:40%%; border-radius:999px; background:#fff; animation:slide 1.6s ease-in-out infinite; }
  @keyframes slide { 0%%{transform:translateX(-100%%)} 100%%{transform:translateX(320%%)} }
  @media (max-width:480px){ .card{padding:28px 20px} h1{font-size:19px} }
</style>
</head>
<body>
  <div class="card">
    <div class="icon">🛠</div>
    <h1>系统维护中</h1>
    <p>%s</p>
    <div class="bar"><i></i></div>
    <div class="tip">页面每 20 秒自动刷新，维护完成后即可正常访问</div>
  </div>
  <script>
    setTimeout(function(){ location.reload(); }, 20000);
  </script>
</body>
</html>`, esc)
}
