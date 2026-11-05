package main

// updater.go 实现 macOS 桌面端自助更新：
//
//	检查 GitHub Releases（tag 前缀 desktop-v）→ 选择当前芯片架构的 zip
//	→ 下载并校验 sha256 → 解包 → 校验 bundle id / 版本 / 代码签名
//	→ 替换当前 .app → 重启。
//
// 设计约束：
//   - 只信任 HTTPS；下载完成后必须与 Release asset 的 sha256 一致；
//   - 安装前校验 Info.plist 的 bundle id、版本号与 codesign 签名；
//   - 启动时只「检查 + 提示」，安装必须由用户在设置页点击触发；
//   - 不依赖 Sparkle（Go + Wails 应用里引入 Swift 框架成本高），
//     替换逻辑用文件系统 rename 完成，失败会尝试回滚；
//   - CX_UPDATE_API_BASE 可覆盖 API 地址（本地测试 / 镜像用），
//     CX_NO_UPDATE_CHECK=1 可关闭启动自动检查。

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	updateRepoSlug         = "songoao25/chaoxing"
	updateTagPrefix        = "desktop-v"
	updateReleasesPerPage  = 30
	updateCheckTimeout     = 15 * time.Second
	updateDownloadTimeout  = 10 * time.Minute
	updateExpectedBundleID = "com.wails.chaoxing-desktop"
	updateOldBundlePrefix  = ".chaoxing-old-"
	updateStagePrefix      = ".chaoxing-stage-"
	updateUserAgentPrefix  = "chaoxing-desktop/"
)

var (
	// errUpdateUnsupported 表示当前平台还没有自动更新实现。
	errUpdateUnsupported = errors.New("自动更新目前仅支持 macOS")
	// errUpdateNoAsset 表示最新 Release 里没有匹配当前架构的安装包。
	errUpdateNoAsset = errors.New("最新版本没有提供当前芯片架构的安装包")
)

// UpdateInfo 是检查更新的内部结果（AssetURL 不下发前端）。
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	Tag            string
	ReleaseURL     string
	Notes          string
	PublishedAt    string
	AssetName      string
	AssetURL       string
	AssetSize      int64
	Digest         string
	Available      bool
	NoReleases     bool
	CheckedAt      time.Time
}

type githubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

// updateDeps 收集外部依赖，便于测试注入。
type updateDeps struct {
	apiBase string
	client  *http.Client
	goos    string
	goarch  string
	runCmd  func(name string, args ...string) ([]byte, error)
}

func defaultUpdateDeps() updateDeps {
	base := strings.TrimSpace(os.Getenv("CX_UPDATE_API_BASE"))
	if base == "" {
		base = "https://api.github.com"
	}
	return updateDeps{
		apiBase: strings.TrimRight(base, "/"),
		client:  &http.Client{Timeout: updateCheckTimeout},
		goos:    runtime.GOOS,
		goarch:  runtime.GOARCH,
		runCmd:  execCommandOutput,
	}
}

func execCommandOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// updateAutoCheckEnabled 是否在启动时自动检查更新。
func updateAutoCheckEnabled() bool {
	v := strings.TrimSpace(os.Getenv("CX_NO_UPDATE_CHECK"))
	return !(v == "1" || strings.EqualFold(v, "true"))
}

// updateAssetName 返回当前平台/架构对应的 Release 资源名。
func updateAssetName(goos, goarch string) (string, bool) {
	if goos != "darwin" {
		return "", false
	}
	switch goarch {
	case "arm64":
		return "chaoxing-desktop-macos-arm64.zip", true
	case "amd64":
		return "chaoxing-desktop-macos-amd64.zip", true
	default:
		return "", false
	}
}

// semverParts 解析 0.2.0 / desktop-v0.2.0 / v0.2.0 为数字段。
func semverParts(s string) ([]int, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, updateTagPrefix)
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return nil, false
	}
	out := make([]int, 0, 3)
	for _, part := range strings.Split(s, ".") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// compareSemver 比较两个版本号：a>b 返回 1，a<b 返回 -1，相等 0。
func compareSemver(a, b string) (int, bool) {
	pa, okA := semverParts(a)
	pb, okB := semverParts(b)
	if !okA || !okB {
		return 0, false
	}
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(pa) {
			av = pa[i]
		}
		if i < len(pb) {
			bv = pb[i]
		}
		if av > bv {
			return 1, true
		}
		if av < bv {
			return -1, true
		}
	}
	return 0, true
}

// fetchReleases 拉取仓库最近的 Release 列表。
func fetchReleases(ctx context.Context, client *http.Client, apiBase string) ([]githubRelease, error) {
	if client == nil {
		client = &http.Client{Timeout: updateCheckTimeout}
	}
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=%d", strings.TrimRight(apiBase, "/"), updateRepoSlug, updateReleasesPerPage)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("无法创建更新检查请求：%v", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", updateUserAgentPrefix+appVersion)
	// 可选：CX_UPDATE_TOKEN 用更高限额的 GitHub token（不写入日志）。
	if token := strings.TrimSpace(os.Getenv("CX_UPDATE_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接更新服务器：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var apiErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			return nil, errors.New("GitHub 更新接口访问受限（可能是限流），请稍后再试")
		}
		if detail := strings.TrimSpace(apiErr.Message); detail != "" {
			return nil, fmt.Errorf("更新服务器返回状态 %d：%s", resp.StatusCode, detail)
		}
		return nil, fmt.Errorf("更新服务器返回状态 %d", resp.StatusCode)
	}
	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析更新信息失败：%v", err)
	}
	return releases, nil
}

// checkForUpdate 查询最新稳定版并判断是否比当前版本新。
func checkForUpdate(ctx context.Context, deps updateDeps, currentVersion string) (UpdateInfo, error) {
	info := UpdateInfo{CurrentVersion: currentVersion, CheckedAt: time.Now()}
	assetName, ok := updateAssetName(deps.goos, deps.goarch)
	if !ok {
		return info, errUpdateUnsupported
	}
	releases, err := fetchReleases(ctx, deps.client, deps.apiBase)
	if err != nil {
		return info, err
	}

	var chosen *githubRelease
	for i := range releases {
		rel := &releases[i]
		if rel.Draft || rel.Prerelease {
			continue
		}
		if !strings.HasPrefix(rel.TagName, updateTagPrefix) {
			continue
		}
		if _, ok := semverParts(rel.TagName); !ok {
			continue
		}
		if chosen == nil {
			chosen = rel
			continue
		}
		if cmp, ok := compareSemver(rel.TagName, chosen.TagName); ok && cmp > 0 {
			chosen = rel
		}
	}
	if chosen == nil {
		info.NoReleases = true
		return info, nil
	}

	info.LatestVersion = strings.TrimPrefix(chosen.TagName, updateTagPrefix)
	info.Tag = chosen.TagName
	info.ReleaseURL = chosen.HTMLURL
	info.Notes = strings.TrimSpace(chosen.Body)
	if !chosen.PublishedAt.IsZero() {
		info.PublishedAt = chosen.PublishedAt.Local().Format("2006-01-02 15:04")
	}

	asset, ok := pickReleaseAsset(chosen.Assets, assetName)
	if !ok {
		return info, errUpdateNoAsset
	}
	info.AssetName = asset.Name
	info.AssetURL = asset.BrowserDownloadURL
	info.AssetSize = asset.Size
	info.Digest = strings.TrimSpace(asset.Digest)

	cmp, ok := compareSemver(info.LatestVersion, currentVersion)
	if !ok {
		return info, fmt.Errorf("无法比较版本号：最新 %q / 当前 %q", info.LatestVersion, currentVersion)
	}
	info.Available = cmp > 0
	return info, nil
}

// pickReleaseAsset 在 Release 资源里找同名安装包。
func pickReleaseAsset(assets []githubAsset, want string) (githubAsset, bool) {
	for _, asset := range assets {
		if asset.Name == want {
			return asset, true
		}
	}
	return githubAsset{}, false
}

// downloadFile 下载资源并校验 sha256；digest 形如 "sha256:abcd..."。
func downloadFile(ctx context.Context, client *http.Client, url, destPath, expectedDigest string, expectedSize int64, onProgress func(downloaded, total int64)) error {
	if !strings.HasPrefix(expectedDigest, "sha256:") || len(expectedDigest) <= len("sha256:") {
		return errors.New("发布资源缺少 sha256 校验值，已停止更新")
	}
	wantHash := strings.TrimPrefix(expectedDigest, "sha256:")
	if client == nil {
		client = &http.Client{Timeout: updateDownloadTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("无法创建下载请求：%v", err)
	}
	req.Header.Set("User-Agent", updateUserAgentPrefix+appVersion)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载更新包失败：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载更新包失败：服务器返回 %d", resp.StatusCode)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = expectedSize
	}
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("无法写入更新包：%v", err)
	}
	defer f.Close()

	hasher := sha256.New()
	buf := make([]byte, 256*1024)
	var written int64
	lastReport := time.Now()
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return fmt.Errorf("写入更新包失败：%v", err)
			}
			hasher.Write(buf[:n])
			written += int64(n)
			if onProgress != nil && (time.Since(lastReport) > 120*time.Millisecond || written == total) {
				onProgress(written, total)
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("下载更新包中断：%v", readErr)
		}
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("写入更新包失败：%v", err)
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(got, wantHash) {
		return errors.New("更新包校验失败（sha256 不匹配），已停止更新")
	}
	return nil
}

// extractZipArchive 把 zip 解压到 destDir；拒绝绝对路径与 ../ 穿越。
func extractZipArchive(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("更新包不是有效 zip：%v", err)
	}
	defer reader.Close()
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		name := file.Name
		if strings.HasPrefix(name, "__MACOSX/") || name == ".DS_Store" {
			continue
		}
		target := filepath.Join(destAbs, filepath.FromSlash(name))
		if !strings.HasPrefix(target, destAbs+string(os.PathSeparator)) && target != destAbs {
			return fmt.Errorf("更新包包含非法路径：%s", name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("创建目录失败：%v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("创建目录失败：%v", err)
		}
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("读取更新包失败：%v", err)
		}
		mode := file.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			src.Close()
			return fmt.Errorf("写入文件失败：%v", err)
		}
		_, copyErr := io.Copy(dst, io.LimitReader(src, 1<<30))
		src.Close()
		dst.Close()
		if copyErr != nil {
			return fmt.Errorf("解压失败：%v", copyErr)
		}
	}
	return nil
}

// findAppBundle 在解压目录中找第一个 .app（要求含 Contents/Info.plist）。
func findAppBundle(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("读取更新包内容失败：%v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".app") {
			continue
		}
		candidate := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(candidate, "Contents", "Info.plist")); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("更新包里没有找到可安装的 .app")
}

// validateUpdateBundle 校验 bundle id、版本号与代码签名。
func validateUpdateBundle(appPath, expectedVersion string, runCmd func(name string, args ...string) ([]byte, error)) error {
	if runCmd == nil {
		runCmd = execCommandOutput
	}
	infoPlist := filepath.Join(appPath, "Contents", "Info.plist")
	if _, err := os.Stat(infoPlist); err != nil {
		return errors.New("更新包结构不完整：缺少 Contents/Info.plist")
	}
	bundleID, err := runCmd("plutil", "-extract", "CFBundleIdentifier", "raw", "-o", "-", infoPlist)
	if err != nil {
		return fmt.Errorf("无法读取更新包标识：%v", err)
	}
	if strings.TrimSpace(string(bundleID)) != updateExpectedBundleID {
		return fmt.Errorf("更新包标识不匹配（%s），已停止更新", strings.TrimSpace(string(bundleID)))
	}
	versionOut, err := runCmd("plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-", infoPlist)
	if err != nil {
		return fmt.Errorf("无法读取更新包版本：%v", err)
	}
	gotVersion := strings.TrimSpace(string(versionOut))
	if expectedVersion != "" && gotVersion != expectedVersion {
		return fmt.Errorf("更新包版本不一致（%s ≠ %s），已停止更新", gotVersion, expectedVersion)
	}
	if _, err := runCmd("codesign", "--verify", "--deep", "--strict", appPath); err != nil {
		return fmt.Errorf("更新包签名校验失败：%v", err)
	}
	return nil
}

// currentAppBundlePath 返回当前进程所在 .app 路径。
func currentAppBundlePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法定位当前 App：%v", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// <App>.app/Contents/MacOS/<binary>
	contents := filepath.Dir(filepath.Dir(exe))
	app := filepath.Dir(contents)
	if !strings.HasSuffix(app, ".app") {
		return "", errors.New("当前程序不在 .app 内，无法自动更新；请手动下载安装包")
	}
	return app, nil
}

// installAppBundle 用 stagedApp 替换 currentApp：先备份旧包再换入，失败尝试回滚。
// 返回备份路径，调用方在启动新版本后清理。
func installAppBundle(currentApp, stagedApp string) (string, error) {
	parent := filepath.Dir(currentApp)
	if parent == "" || parent == "." {
		return "", errors.New("无法确定应用安装目录")
	}
	testDir, err := os.MkdirTemp(parent, ".chaoxing-write-test-")
	if err != nil {
		return "", fmt.Errorf("没有权限写入应用目录（%s）：请把 App 拖到有写权限的位置后重试", parent)
	}
	_ = os.RemoveAll(testDir)

	backup := filepath.Join(parent, updateOldBundlePrefix+time.Now().Format("20060102-150405")+"-"+filepath.Base(currentApp))
	if err := os.Rename(currentApp, backup); err != nil {
		return "", fmt.Errorf("无法替换旧版本：%v", err)
	}
	if err := os.Rename(stagedApp, currentApp); err != nil {
		_ = os.Rename(backup, currentApp) // 回滚
		return "", fmt.Errorf("安装新版本失败：%v", err)
	}
	return backup, nil
}

// runUpdatePipeline 执行「下载 → 校验 → 解包 → 校验 bundle → 替换 → 启动」的完整流程。
// 抽成独立函数是为了可测试：测试可以传入临时 app 路径与假的 launch。
// 返回旧版本备份路径，调用方负责在启动新版本后清理。
func runUpdatePipeline(ctx context.Context, deps updateDeps, info UpdateInfo, appPath string, onProgress func(UpdateProgress), launch func(string) error) (string, error) {
	if onProgress == nil {
		onProgress = func(UpdateProgress) {}
	}
	parent := filepath.Dir(appPath)
	stageDir, err := os.MkdirTemp(parent, updateStagePrefix+"*")
	if err != nil {
		return "", fmt.Errorf("无法在应用目录创建临时文件夹：%v", err)
	}
	defer func() { _ = os.RemoveAll(stageDir) }()

	zipPath := filepath.Join(stageDir, "update.zip")
	client := &http.Client{Timeout: updateDownloadTimeout}
	onProgress(UpdateProgress{Phase: "download", Message: "正在下载更新…"})
	if err := downloadFile(ctx, client, info.AssetURL, zipPath, info.Digest, info.AssetSize,
		func(downloaded, total int64) {
			percent := 0
			if total > 0 {
				percent = int(downloaded * 100 / total)
				if percent > 100 {
					percent = 100
				}
			}
			onProgress(UpdateProgress{Phase: "download", Downloaded: downloaded, Total: total, Percent: percent})
		}); err != nil {
		return "", err
	}

	onProgress(UpdateProgress{Phase: "verify", Percent: 100, Message: "正在校验更新包…"})
	unpackDir := filepath.Join(stageDir, "unpack")
	if err := extractZipArchive(zipPath, unpackDir); err != nil {
		return "", err
	}
	stagedApp, err := findAppBundle(unpackDir)
	if err != nil {
		return "", err
	}
	if err := validateUpdateBundle(stagedApp, info.LatestVersion, deps.runCmd); err != nil {
		return "", err
	}

	onProgress(UpdateProgress{Phase: "install", Percent: 100, Message: "正在安装新版本…"})
	backup, err := installAppBundle(appPath, stagedApp)
	if err != nil {
		return "", err
	}

	onProgress(UpdateProgress{Phase: "restart", Percent: 100, Message: "更新完成，正在重启…"})
	if launch != nil {
		if err := launch(appPath); err != nil {
			_ = os.RemoveAll(backup)
			return "", fmt.Errorf("新版本已就位，但启动失败：%v；请手动打开 %s", err, appPath)
		}
	}
	return backup, nil
}

// cleanupStaleUpdateArtifacts 清理上次更新遗留的备份/临时目录（启动时调用）。
func cleanupStaleUpdateArtifacts(appPath string) {
	parent := filepath.Dir(appPath)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, updateOldBundlePrefix) && !strings.HasPrefix(name, updateStagePrefix) {
			continue
		}
		if entry.IsDir() || strings.HasSuffix(name, ".app") {
			_ = os.RemoveAll(filepath.Join(parent, name))
		}
	}
}
