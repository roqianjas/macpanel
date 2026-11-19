package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/roqianjas/macpanel/internal/accounts"
	"github.com/roqianjas/macpanel/internal/auth"
	"github.com/roqianjas/macpanel/internal/backup"
	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/crypto"
	"github.com/roqianjas/macpanel/internal/databases"
	"github.com/roqianjas/macpanel/internal/deploy"
	"github.com/roqianjas/macpanel/internal/diskclean"
	"github.com/roqianjas/macpanel/internal/docker"
	"github.com/roqianjas/macpanel/internal/docs"
	"github.com/roqianjas/macpanel/internal/exposure"
	"github.com/roqianjas/macpanel/internal/hardware"
	"github.com/roqianjas/macpanel/internal/jobs"
	"github.com/roqianjas/macpanel/internal/netmon"
	"github.com/roqianjas/macpanel/internal/overview"
	"github.com/roqianjas/macpanel/internal/phpruntime"
	"github.com/roqianjas/macpanel/internal/redis"
	"github.com/roqianjas/macpanel/internal/services"
	"github.com/roqianjas/macpanel/internal/settings"
	"github.com/roqianjas/macpanel/internal/sites"
	"github.com/roqianjas/macpanel/internal/store"
	"github.com/roqianjas/macpanel/internal/system"
	"github.com/roqianjas/macpanel/internal/tasks"
	"github.com/roqianjas/macpanel/internal/updates"
	"github.com/roqianjas/macpanel/internal/web"
)

// hRoutes: daftar rute yang bisa dipanggil lewat token (untuk agen). min_scope = tingkat minimum; aksi yang
// butuh verifikasi ulang hanya berhasil dengan token "full".
func hRoutes(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	type row struct {
		Method   string `json:"method"`
		Path     string `json:"path"`
		MinScope string `json:"min_scope"`
	}
	out := []row{}
	for _, rt := range Routes() {
		if rt.Level != core.Full || !core.TokenAllowed(rt.Method, rt.Path) {
			continue
		}
		m := "operate"
		if rt.Method == "GET" || rt.Method == "HEAD" {
			m = "read"
		}
		out = append(out, row{rt.Method, rt.Path, m})
	}
	core.WriteJSON(w, 200, map[string]any{"routes": out, "note": "Aksi bertanda step-up (hapus, config, pasang, ...) butuh token 'full'; penghapusan wajib menyertakan confirm=<nama>. Rute /api/auth, /api/me/*, */creds, */env tidak tersedia lewat token."})
}

func NewApp(st *store.Store, key []byte, cfg core.Config) *core.App {
	home, _ := os.UserHomeDir()
	root := os.Getenv("MACPANEL_HOME")
	if root == "" {
		root = filepath.Join(home, "MacPanel")
	}
	return &core.App{Store: st, Key: key, Cfg: cfg, Dummy: crypto.HashPassword("dummy-untuk-menyamakan-waktu"), Now: time.Now,
		LabBin: "/opt/homebrew/bin/lab", Home: home, Root: root, Wwwroot: filepath.Join(root, "wwwroot"), SitesDir: filepath.Join(root, "server/nginx/sites"), TLD: "lab", ProbeAddr: core.EnvOr("MACPANEL_PROBE_ADDR", "127.0.0.1:443"), Timeout: 120 * time.Second, LongTimeout: 30 * time.Minute,
		NginxDir: filepath.Join(root, "server/nginx"), CertDir: "/opt/homebrew/etc/nginx/certs", LogDir: "/opt/homebrew/var/log/nginx/sites", ErrLog: "/opt/homebrew/var/log/nginx/error.log"}
}

// Routes adalah SATU-SATUNYA daftar rute. Tes memastikan semua yang bukan Public menolak permintaan tanpa sesi.
func Routes() []core.Route {
	return []core.Route{
		{Method: "GET", Path: "/healthz", Level: core.Public, H: system.Health},
		{Method: "POST", Path: "/api/auth/login", Level: core.Public, H: auth.Login},
		{Method: "POST", Path: "/api/auth/totp", Level: core.Pwd, H: auth.TOTP},
		{Method: "POST", Path: "/api/auth/logout", Level: core.Any, H: auth.Logout},
		{Method: "POST", Path: "/api/auth/stepup", Level: core.Full, H: auth.StepUp},
		{Method: "GET", Path: "/api/me", Level: core.Full, H: auth.Me},
		{Method: "GET", Path: "/api/settings/public", Level: core.Public, H: settings.PanelSettingsPublic},
		{Method: "GET", Path: "/api/settings", Level: core.Full, H: settings.PanelSettingsGet},
		{Method: "POST", Path: "/api/settings", Level: core.Full, H: settings.PanelSettingsSet},
		{Method: "GET", Path: "/api/routes", Level: core.Full, H: hRoutes},
		{Method: "GET", Path: "/api/me/2fa", Level: core.Full, H: auth.Get2FA},
		{Method: "POST", Path: "/api/me/2fa/setup", Level: core.Full, H: auth.Setup2FA},
		{Method: "POST", Path: "/api/me/2fa/enable", Level: core.Full, H: auth.Enable2FA},
		{Method: "POST", Path: "/api/me/2fa/disable", Level: core.Full, H: auth.Disable2FA},
		{Method: "GET", Path: "/api/me/tokens", Level: core.Full, H: auth.TokensList},
		{Method: "POST", Path: "/api/me/tokens", Level: core.Full, H: auth.TokenCreate},
		{Method: "POST", Path: "/api/me/tokens/revoke", Level: core.Full, H: auth.TokenRevoke},
		{Method: "GET", Path: "/api/audit", Level: core.Full, H: auth.Audit},
		{Method: "GET", Path: "/api/sites", Level: core.Full, H: sites.SitesRich},
		{Method: "POST", Path: "/api/sites", Level: core.Full, H: sites.SiteAdd},
		{Method: "POST", Path: "/api/sites/{name}/remove", Level: core.Full, H: sites.SiteRemove},
		{Method: "GET", Path: "/api/db", Level: core.Full, H: databases.DBList},
		{Method: "POST", Path: "/api/db", Level: core.Full, H: databases.DBCreate},
		{Method: "POST", Path: "/api/db/{engine}/{name}/dump", Level: core.Full, H: databases.DBDump},
		{Method: "POST", Path: "/api/db/{engine}/{name}/drop", Level: core.Full, H: databases.DBDrop},
		{Method: "GET", Path: "/api/jobs", Level: core.Full, H: jobs.Jobs},
		{Method: "GET", Path: "/api/sites/{name}/settings", Level: core.Full, H: sites.SettingsGet},
		{Method: "POST", Path: "/api/sites/{name}/settings", Level: core.Full, H: sites.SettingsSet},
		{Method: "GET", Path: "/api/sites/{name}/config", Level: core.Full, H: sites.ConfigGet},
		{Method: "POST", Path: "/api/sites/{name}/config", Level: core.Full, H: sites.ConfigSet},
		{Method: "POST", Path: "/api/sites/{name}/restore", Level: core.Full, H: sites.Restore},
		{Method: "GET", Path: "/api/sites/{name}/ssl", Level: core.Full, H: sites.SSLGet},
		{Method: "POST", Path: "/api/sites/{name}/ssl/renew", Level: core.Full, H: sites.SSLRenew},
		{Method: "GET", Path: "/api/sites/{name}/logs", Level: core.Full, H: sites.Logs},
		{Method: "GET", Path: "/api/jobs/{id}", Level: core.Full, H: jobs.Job},
		{Method: "GET", Path: "/api/db/{engine}/{name}/dumps", Level: core.Full, H: databases.DBDumps},
		{Method: "POST", Path: "/api/db/{engine}/{name}/restore", Level: core.Full, H: databases.DBRestore},
		{Method: "GET", Path: "/api/php/{version}/ext", Level: core.Full, H: phpruntime.ExtListHandler},
		{Method: "POST", Path: "/api/php/{version}/ext/{name}/{action}", Level: core.Full, H: phpruntime.ExtAction},
		{Method: "GET", Path: "/api/php/{version}/ini", Level: core.Full, H: phpruntime.IniGet},
		{Method: "POST", Path: "/api/php/{version}/ini", Level: core.Full, H: phpruntime.IniSet},
		{Method: "GET", Path: "/api/composer", Level: core.Full, H: phpruntime.Composer},
		{Method: "POST", Path: "/api/composer/{action}", Level: core.Full, H: phpruntime.ComposerAct},
		{Method: "GET", Path: "/api/sites/{name}/deploy", Level: core.Full, H: deploy.DeployInfoHandler},
		{Method: "POST", Path: "/api/sites/{name}/deploy", Level: core.Full, H: deploy.DeployRun},
		{Method: "GET", Path: "/api/backups", Level: core.Full, H: backup.Backups},
		{Method: "GET", Path: "/api/alerts", Level: core.Full, H: overview.Alerts},
		{Method: "GET", Path: "/api/sites/{name}/hook", Level: core.Full, H: deploy.HookGet},
		{Method: "POST", Path: "/api/sites/{name}/hook", Level: core.Full, H: deploy.HookSet},
		{Method: "POST", Path: "/api/sites/{name}/hook/remove", Level: core.Full, H: deploy.HookDel},
		{Method: "GET", Path: "/api/backups/prune-preview", Level: core.Full, H: backup.BackupPrunePreview},
		{Method: "POST", Path: "/api/backups/prune", Level: core.Full, H: backup.BackupPrune},
		{Method: "POST", Path: "/api/backups/delete", Level: core.Full, H: backup.BackupDelete},
		{Method: "GET", Path: "/api/backups/extras", Level: core.Full, H: backup.BackupExtras},
		{Method: "GET", Path: "/api/backups/extras-preview", Level: core.Full, H: backup.BackupExtrasPreview},
		{Method: "POST", Path: "/api/backups/extras", Level: core.Full, H: backup.BackupExtrasClean},
		{Method: "POST", Path: "/api/installer", Level: core.Full, H: sites.Install},
		{Method: "POST", Path: "/api/sites/{name}/clone", Level: core.Full, H: sites.Clone},
		{Method: "GET", Path: "/api/redis/info", Level: core.Full, H: redis.RedisInfo},
		{Method: "GET", Path: "/api/redis/scan", Level: core.Full, H: redis.RedisScan},
		{Method: "POST", Path: "/api/redis/get", Level: core.Full, H: redis.RedisGet},
		{Method: "POST", Path: "/api/redis/del", Level: core.Full, H: redis.RedisDel},
		{Method: "POST", Path: "/api/me/lang", Level: core.Full, H: auth.MyLang},
		{Method: "POST", Path: "/api/me/profile", Level: core.Full, H: auth.MyProfile},
		{Method: "POST", Path: "/api/me/password", Level: core.Full, H: auth.MyPassword},
		{Method: "GET", Path: "/api/me/devices", Level: core.Full, H: auth.Devices},
		{Method: "GET", Path: "/api/docs", Level: core.Full, H: docs.Docs},
		{Method: "POST", Path: "/api/me/devices/revoke", Level: core.Full, H: auth.DeviceRevoke},
		{Method: "GET", Path: "/api/sys/procs", Level: core.Full, H: system.SysProcs},
		{Method: "POST", Path: "/api/sys/kill", Level: core.Full, H: system.SysKill},
		{Method: "GET", Path: "/api/sys/disk", Level: core.Full, H: system.SysDisk},
		{Method: "GET", Path: "/api/sys/security", Level: core.Full, H: system.SysSecurity},
		{Method: "GET", Path: "/api/sys/docker", Level: core.Full, H: docker.SysDocker},
		{Method: "GET", Path: "/api/docker", Level: core.Full, H: docker.DockerOverview},
		{Method: "GET", Path: "/api/docker/stats", Level: core.Full, H: docker.DockerStats},
		{Method: "GET", Path: "/api/docker/containers/{id}/logs", Level: core.Full, H: docker.DockerLogs},
		{Method: "POST", Path: "/api/docker/containers/{id}/{action}", Level: core.Full, H: docker.DockerContainer},
		{Method: "POST", Path: "/api/docker/images/pull", Level: core.Full, H: docker.DockerPull},
		{Method: "POST", Path: "/api/docker/images/remove", Level: core.Full, H: docker.DockerImageRemove},
		{Method: "POST", Path: "/api/docker/images/prune", Level: core.Full, H: docker.DockerPrune},
		{Method: "GET", Path: "/api/docker/compose/{project}/ps", Level: core.Full, H: docker.DockerComposePS},
		{Method: "GET", Path: "/api/docker/compose/{project}/logs", Level: core.Full, H: docker.DockerComposeLogs},
		{Method: "POST", Path: "/api/docker/compose/{project}/{action}", Level: core.Full, H: docker.DockerComposeAction},
		{Method: "GET", Path: "/api/sys/dns", Level: core.Full, H: system.SysDNS},
		{Method: "GET", Path: "/api/sites/{name}/stats", Level: core.Full, H: sites.SiteStats},
		{Method: "GET", Path: "/api/db/{engine}/{name}/users", Level: core.Full, H: databases.DBUsers},
		{Method: "POST", Path: "/api/db/{engine}/{name}/passwd", Level: core.Full, H: databases.DBPasswd},
		{Method: "POST", Path: "/api/db/{engine}/{name}/creds", Level: core.Full, H: databases.DBCreds},
		{Method: "GET", Path: "/api/backups/config", Level: core.Full, H: backup.BackupCfgGet},
		{Method: "POST", Path: "/api/backups/config", Level: core.Full, H: backup.BackupCfgSet},
		{Method: "GET", Path: "/api/backups/ls", Level: core.Full, H: backup.BackupLs},
		{Method: "POST", Path: "/api/backups/extract", Level: core.Full, H: backup.BackupExtract},
		{Method: "GET", Path: "/api/sites/{name}/auth", Level: core.Full, H: sites.AuthGet},
		{Method: "POST", Path: "/api/sites/{name}/auth/users", Level: core.Full, H: sites.AuthSet},
		{Method: "POST", Path: "/api/sites/{name}/auth/users/{user}/remove", Level: core.Full, H: sites.AuthDel},
		{Method: "GET", Path: "/api/sites/{name}/env", Level: core.Full, H: sites.EnvGet},
		{Method: "POST", Path: "/api/sites/{name}/env", Level: core.Full, H: sites.EnvSet},
		{Method: "GET", Path: "/api/dbcfg/{engine}", Level: core.Full, H: databases.DBCfgGet},
		{Method: "POST", Path: "/api/dbcfg/{engine}", Level: core.Full, H: databases.DBCfgSet},
		{Method: "GET", Path: "/api/dbcfg/{engine}/slow", Level: core.Full, H: databases.SlowLog},
		{Method: "GET", Path: "/api/users", Level: core.Full, H: accounts.Users},
		{Method: "GET", Path: "/api/accounts", Level: core.Full, H: accounts.Accounts},
		{Method: "POST", Path: "/api/accounts", Level: core.Full, H: accounts.AccountCreate},
		{Method: "POST", Path: "/api/accounts/{name}/role", Level: core.Full, H: accounts.AccountRole},
		{Method: "POST", Path: "/api/accounts/{name}/disable", Level: core.Full, H: accounts.AccountDisable},
		{Method: "POST", Path: "/api/accounts/{name}/enable", Level: core.Full, H: accounts.AccountEnable},
		{Method: "POST", Path: "/api/accounts/{name}/reset", Level: core.Full, H: accounts.AccountReset},
		{Method: "POST", Path: "/api/accounts/{name}/2fa-off", Level: core.Full, H: accounts.Account2FAOff},
		{Method: "POST", Path: "/api/accounts/{name}/remove", Level: core.Full, H: accounts.AccountRemove},
		{Method: "GET", Path: "/api/db/{engine}/{name}/dbusers", Level: core.Full, H: databases.DBUserList},
		{Method: "POST", Path: "/api/db/{engine}/{name}/dbusers", Level: core.Full, H: databases.DBUserAdd},
		{Method: "POST", Path: "/api/db/{engine}/{name}/dbusers/{user}/passwd", Level: core.Full, H: databases.DBUserPasswd},
		{Method: "POST", Path: "/api/db/{engine}/{name}/dbusers/{user}/privs", Level: core.Full, H: databases.DBUserPrivs},
		{Method: "POST", Path: "/api/db/{engine}/{name}/dbusers/{user}/remove", Level: core.Full, H: databases.DBUserRemove},
		{Method: "POST", Path: "/api/dbroot/{engine}/passwd", Level: core.Full, H: databases.DBRootPasswd},
		{Method: "POST", Path: "/api/db/{engine}/{name}/upload", Level: core.Full, H: databases.DBUpload},
		{Method: "POST", Path: "/api/backups/run", Level: core.Full, H: backup.BackupRun},
		{Method: "GET", Path: "/api/services", Level: core.Full, H: services.ServicesList},
		{Method: "POST", Path: "/api/services/{id}/{action}", Level: core.Full, H: services.ServiceAction},
		{Method: "GET", Path: "/api/services/{id}/logs", Level: core.Full, H: services.ServiceLogs},
		{Method: "POST", Path: "/api/apps/{id}/install", Level: core.Full, H: services.AppInstall},
		{Method: "POST", Path: "/api/apps/{id}/remove", Level: core.Full, H: services.AppRemove},
		{Method: "POST", Path: "/api/sites/{name}/{action}", Level: core.Full, H: sites.SiteToggle},
		{Method: "GET", Path: "/api/metrics", Level: core.Full, H: system.Metrics},
		{Method: "GET", Path: "/api/hw", Level: core.Full, H: hardware.Hardware},
		{Method: "GET", Path: "/api/hw/thresholds", Level: core.Full, H: hardware.ThresholdsGet},
		{Method: "GET", Path: "/api/exposure", Level: core.Full, H: exposure.Handler},
		{Method: "GET", Path: "/api/tailscale", Level: core.Full, H: system.TailscaleStatus},
		{Method: "GET", Path: "/api/clean", Level: core.Full, H: diskclean.Last},
		{Method: "POST", Path: "/api/clean/scan", Level: core.Full, H: diskclean.Scan},
		{Method: "POST", Path: "/api/clean/preview", Level: core.Full, H: diskclean.Preview},
		{Method: "POST", Path: "/api/clean/run", Level: core.Full, H: diskclean.Run},
		{Method: "GET", Path: "/api/mem", Level: core.Full, H: diskclean.Mem},
		{Method: "POST", Path: "/api/exposure/allow", Level: core.Full, H: exposure.Change(true)},
		{Method: "POST", Path: "/api/exposure/revoke", Level: core.Full, H: exposure.Change(false)},
		{Method: "POST", Path: "/api/hw/thresholds", Level: core.Full, H: hardware.ThresholdsSet},
		{Method: "GET", Path: "/api/netmon", Level: core.Full, H: netmon.Handler},
		{Method: "GET", Path: "/api/notify", Level: core.Full, H: overview.NotifyGet},
		{Method: "POST", Path: "/api/notify", Level: core.Full, H: overview.NotifySet},
		{Method: "POST", Path: "/api/notify/test", Level: core.Full, H: overview.NotifyTest},
		{Method: "GET", Path: "/api/system", Level: core.Full, H: system.System},
		{Method: "GET", Path: "/api/php", Level: core.Full, H: phpruntime.PHPList},
		{Method: "POST", Path: "/api/php/{version}/install", Level: core.Full, H: phpruntime.PHPInstall},
		{Method: "POST", Path: "/api/php/{version}/remove", Level: core.Full, H: phpruntime.PHPRemove},
		{Method: "GET", Path: "/api/runtime", Level: core.Full, H: phpruntime.RuntimeList},
		{Method: "POST", Path: "/api/runtime/{tool}/{version}/install", Level: core.Full, H: phpruntime.RuntimeInstall},
		{Method: "POST", Path: "/api/runtime/{tool}/{version}/use", Level: core.Full, H: phpruntime.RuntimeUse},
		{Method: "POST", Path: "/api/runtime/{tool}/{version}/remove", Level: core.Full, H: phpruntime.RuntimeRemove},
		{Method: "GET", Path: "/api/tasks/{kind}", Level: core.Full, H: tasks.TasksList},
		{Method: "POST", Path: "/api/tasks/{kind}", Level: core.Full, H: tasks.TaskAdd},
		{Method: "POST", Path: "/api/tasks/{kind}/{name}/restart", Level: core.Full, H: tasks.TaskRestart},
		{Method: "POST", Path: "/api/tasks/{kind}/{name}/remove", Level: core.Full, H: tasks.TaskRemove},
		{Method: "GET", Path: "/api/tasks/{kind}/{name}/logs", Level: core.Full, H: tasks.TaskLogs},
		{Method: "GET", Path: "/api/updates", Level: core.Full, H: updates.UpdatesCheck},
		{Method: "POST", Path: "/api/updates/apply", Level: core.Full, H: updates.UpdatesApply},
	}
}

func Handler(a *core.App) http.Handler {
	mux := http.NewServeMux()
	for _, rt := range Routes() {
		mux.HandleFunc(rt.Method+" "+rt.Path, a.Wrap(rt))
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			core.SecHeaders(w)
			core.WriteJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		web.ServeStatic(a, w, r)
	})
	return mux
}
