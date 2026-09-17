package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"iptv-spider-sh/global"
	"iptv-spider-sh/model"
	"iptv-spider-sh/modules/auth"
	"iptv-spider-sh/modules/cronmgr"
	"iptv-spider-sh/modules/loghub"
	"iptv-spider-sh/modules/reqlog"

	"github.com/kataras/iris/v12"
)

// ============================ /api/health ============================

type healthResp struct {
	Status         string `json:"status"`
	Version        string `json:"version"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	DbType         string `json:"db_type"`
	DbOK           bool   `json:"db_ok"`
	AuthSessionOK  bool   `json:"auth_session_ok"`
	AuthUID        string `json:"auth_uid"`
	AuthUpdatedAt  string `json:"auth_updated_at"`
	EpgHostURL     string `json:"epg_host_url"`
	ChannelsTotal  int64  `json:"channels_total"`
	ChannelsActive int64  `json:"channels_active"`
	EpgPrograms    int64  `json:"epg_programs"`
	NextFetch      string `json:"next_fetch"`
}

func health(ctx iris.Context) {
	resp := healthResp{
		Status:  "ok",
		Version: global.VERSION,
	}
	if global.CONFIG != nil {
		resp.DbType = global.CONFIG.System.DbType
	}
	if !global.StartedAt.IsZero() {
		resp.UptimeSeconds = int64(time.Since(global.StartedAt).Seconds())
	}

	if global.DB != nil {
		if sqlDB, err := global.DB.DB(); err == nil && sqlDB.Ping() == nil {
			resp.DbOK = true
		}
		global.DB.Model(&model.ChannelInfo{}).Count(&resp.ChannelsTotal)
		global.DB.Model(&model.ChannelInfo{}).Where("is_show = ?", true).Count(&resp.ChannelsActive)
		global.DB.Model(&model.EPGDetails{}).Count(&resp.EpgPrograms)
	}

	if c := auth.GetGlobalClient(); c != nil {
		uid, updatedAt, epgHost, ok := c.Status()
		resp.AuthUID = uid
		resp.EpgHostURL = epgHost
		resp.AuthSessionOK = ok
		if !updatedAt.IsZero() {
			resp.AuthUpdatedAt = updatedAt.Format("2006-01-02 15:04:05")
		}
	}

	if t := cronmgr.NextEPG(); !t.IsZero() {
		resp.NextFetch = t.Format("2006-01-02 15:04:05")
	}

	if !resp.DbOK || !resp.AuthSessionOK {
		resp.Status = "degraded"
	}
	jsonOK(ctx, resp)
}

// ============================ /api/requests ============================

func requests(ctx iris.Context) {
	jsonOK(ctx, reqlog.Recent())
}

// ============================ /api/network-check ============================

type netCheckResult struct {
	Name      string `json:"name"`
	Target    string `json:"target"`
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms"`
	Detail    string `json:"detail"`
	LocalAddr string `json:"local_addr,omitempty"`
}

func checkTCP(name, target string, timeout time.Duration) netCheckResult {
	r := netCheckResult{Name: name, Target: target}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, timeout)
	r.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		r.Detail = err.Error()
		return r
	}
	defer conn.Close()
	r.OK = true
	if la := conn.LocalAddr(); la != nil {
		r.LocalAddr = la.String()
	}
	return r
}

func checkHTTP(name, url string, timeout time.Duration) netCheckResult {
	r := netCheckResult{Name: name, Target: url}
	client := &http.Client{Timeout: timeout}
	start := time.Now()
	resp, err := client.Get(url)
	r.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		r.Detail = err.Error()
		return r
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	// HTTP 能拿到响应即认为链路可达（认证接口裸请求返回「出错页面」属预期）
	r.OK = true
	r.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return r
}

// networkCheck GET /api/network-check
// 同时给出 TCP 连接使用的本地地址，可直观看出流量走了哪块网卡（专网排查常用）。
func networkCheck(ctx iris.Context) {
	results := []netCheckResult{
		checkTCP("外网", "www.baidu.com:80", 5*time.Second),
		checkTCP("IPTV 认证服务器", "222.68.208.73:7001", 5*time.Second),
		checkHTTP("IPTV 认证接口", "http://222.68.208.73:7001/iptv3a/4kLogAuth.do", 8*time.Second),
	}

	if c := auth.GetGlobalClient(); c != nil {
		if host := c.EpgHost(); host != "" {
			results = append(results, checkTCP("EPG 门户", host, 5*time.Second))
		}
	}

	externalOK := results[0].OK
	iptvOK := false
	for _, r := range results[1:] {
		if r.OK {
			iptvOK = true
			break
		}
	}

	jsonOK(ctx, iris.Map{
		"external_ok": externalOK,
		"iptv_ok":     iptvOK,
		"results":     results,
	})
}

// ============================ 版本检查 / 升级 ============================

const defaultVersionRepo = "rayfalling/sh-tel-iptv-spider"

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

type releaseInfo struct {
	Current     string    `json:"current"`
	Latest      string    `json:"latest"`
	HasUpdate   bool      `json:"has_update"`
	ReleaseURL  string    `json:"release_url"`
	PublishedAt string    `json:"published_at"`
	Message     string    `json:"message"`
	Assets      []ghAsset `json:"assets,omitempty"`
}

var (
	releaseMu    sync.Mutex
	releaseCache releaseInfo
	releaseAt    time.Time
)

var versionNumRe = regexp.MustCompile(`\d+(?:\.\d+)*`)

func versionParts(s string) []int {
	m := versionNumRe.FindString(s)
	if m == "" {
		return nil
	}
	fields := strings.Split(m, ".")
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		n, _ := strconv.Atoi(f)
		out = append(out, n)
	}
	return out
}

// versionNewer latest 是否比 current 新
func versionNewer(latest, current string) bool {
	l, c := versionParts(latest), versionParts(current)
	if len(l) == 0 {
		return false
	}
	for i := 0; i < len(l) || i < len(c); i++ {
		var a, b int
		if i < len(l) {
			a = l[i]
		}
		if i < len(c) {
			b = c[i]
		}
		if a != b {
			return a > b
		}
	}
	return false
}

func versionRepo() string {
	if r := strings.TrimSpace(os.Getenv("SPIDER_REPO")); r != "" {
		return r
	}
	return defaultVersionRepo
}

// fetchLatestRelease 查询 GitHub 最新发布（带 10 分钟缓存）
func fetchLatestRelease(force bool) releaseInfo {
	releaseMu.Lock()
	defer releaseMu.Unlock()

	if !force && !releaseAt.IsZero() && time.Since(releaseAt) < 10*time.Minute {
		return releaseCache
	}

	info := releaseInfo{Current: global.VERSION}
	url := "https://api.github.com/repos/" + versionRepo() + "/releases/latest"
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		info.Message = "构造请求失败: " + err.Error()
		return info
	}
	req.Header.Set("User-Agent", "sh-tel-iptv-spider")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		info.Message = "无法访问 GitHub（可能是网络受限）: " + err.Error()
		return info
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		info.Message = fmt.Sprintf("GitHub 返回 HTTP %d", resp.StatusCode)
		return info
	}

	var payload struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		info.Message = "解析 GitHub 响应失败: " + err.Error()
		return info
	}

	info.Latest = payload.TagName
	info.ReleaseURL = payload.HTMLURL
	info.PublishedAt = payload.PublishedAt
	info.HasUpdate = versionNewer(payload.TagName, global.VERSION)
	for _, a := range payload.Assets {
		info.Assets = append(info.Assets, ghAsset{Name: a.Name, URL: a.BrowserDownloadURL, Size: a.Size})
	}

	releaseCache = info
	releaseAt = time.Now()
	return info
}

// versionCheck GET /api/version-check
func versionCheck(ctx iris.Context) {
	info := fetchLatestRelease(ctx.FormValue("ref") == "true")
	jsonOK(ctx, info)
}

// pickAsset 按当前平台挑选发布包
func pickAsset(assets []ghAsset) (ghAsset, bool) {
	want := ""
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		want = "linux_x86_64"
	case "linux/arm64":
		// 上游只提供 OpenWrt 版 aarch64 包，非 OpenWrt 的 arm64 发行版需自行编译
		want = "aarch64_cortex-a53"
	case "windows/amd64":
		want = "windows_x86_64"
	}
	if want == "" {
		return ghAsset{}, false
	}
	for _, a := range assets {
		if strings.Contains(a.Name, want) {
			return a, true
		}
	}
	return ghAsset{}, false
}

// selfUpgrade GET /api/self-upgrade[?confirm=true]
//
// 安全设计：默认只演练（dry-run），不会替换正在运行的二进制。
// 真正执行需要同时满足：环境变量 SPIDER_ALLOW_UPGRADE=true 且请求带 confirm=true。
// 替换前会把旧二进制备份为 <exe>.bak-<时间戳>，替换后需重启进程生效。
func selfUpgrade(ctx iris.Context) {
	confirm := ctx.FormValue("confirm") == "true"
	allowed := strings.EqualFold(os.Getenv("SPIDER_ALLOW_UPGRADE"), "true")

	info := fetchLatestRelease(true)
	if info.Message != "" {
		jsonOK(ctx, iris.Map{"success": false, "message": info.Message})
		return
	}
	if !info.HasUpdate {
		jsonOK(ctx, iris.Map{"success": false, "message": "已是最新版本",
			"current": info.Current, "latest": info.Latest})
		return
	}

	asset, ok := pickAsset(info.Assets)
	if !ok {
		names := make([]string, 0, len(info.Assets))
		for _, a := range info.Assets {
			names = append(names, a.Name)
		}
		jsonOK(ctx, iris.Map{
			"success": false,
			"message": fmt.Sprintf("没有匹配当前平台 %s/%s 的发布包，请手动编译升级", runtime.GOOS, runtime.GOARCH),
			"assets":  names,
		})
		return
	}

	if !confirm || !allowed {
		jsonOK(ctx, iris.Map{
			"success":      false,
			"dry_run":      true,
			"message":      "仅演练，未替换二进制。真正执行需设置环境变量 SPIDER_ALLOW_UPGRADE=true 且请求带 confirm=true",
			"current":      info.Current,
			"latest":       info.Latest,
			"asset":        asset.Name,
			"download_url": asset.URL,
		})
		return
	}

	backup, err := performUpgrade(asset)
	if err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "升级失败: "+err.Error())
		return
	}
	jsonOK(ctx, iris.Map{
		"success": true,
		"message": "二进制已替换，请重启服务生效",
		"backup":  backup,
		"latest":  info.Latest,
	})
}

// performUpgrade 下载发布包 → 解压 → 备份旧文件 → 原子替换
func performUpgrade(asset ghAsset) (string, error) {
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Get(asset.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return "", err
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", fmt.Errorf("发布包不是有效 zip: %w", err)
	}
	if len(zr.File) == 0 {
		return "", errors.New("发布包内没有文件")
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		return "", err
	}
	bin, err := io.ReadAll(io.LimitReader(rc, 256<<20))
	rc.Close()
	if err != nil {
		return "", err
	}
	if len(bin) < 1<<20 {
		return "", fmt.Errorf("解压后仅 %d 字节，疑似不是可执行文件，已放弃替换", len(bin))
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, e := filepath.EvalSymlinks(exe); e == nil {
		exe = resolved
	}

	old, err := os.ReadFile(exe)
	if err != nil {
		return "", fmt.Errorf("读取当前二进制失败: %w", err)
	}
	backup := fmt.Sprintf("%s.bak-%s", exe, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, old, 0755); err != nil {
		return "", fmt.Errorf("备份当前二进制失败: %w", err)
	}

	tmp := exe + ".new"
	if err := os.WriteFile(tmp, bin, 0755); err != nil {
		return "", fmt.Errorf("写入新二进制失败: %w", err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("替换二进制失败: %w", err)
	}

	global.LOG.Info("自升级完成: " + asset.Name + " → " + exe + "（备份 " + backup + "）")
	return backup, nil
}

// ============================ 日志级别 ============================

var logLevels = []string{"debug", "info", "warn", "error"}

func getLogLevel(ctx iris.Context) {
	jsonOK(ctx, iris.Map{"level": loghub.LevelString(), "available": logLevels})
}

func setLogLevel(ctx iris.Context) {
	if !guardWrite(ctx) {
		return
	}
	var req struct {
		Level string `json:"level"`
	}
	if err := ctx.ReadJSON(&req); err != nil || strings.TrimSpace(req.Level) == "" {
		jsonErr(ctx, iris.StatusBadRequest, "参数错误")
		return
	}
	if err := loghub.SetLevel(req.Level); err != nil {
		jsonErr(ctx, iris.StatusBadRequest, err.Error())
		return
	}
	global.LOG.Info("日志级别已切换为: " + loghub.LevelString())
	jsonOK(ctx, iris.Map{"success": true, "level": loghub.LevelString()})
}
