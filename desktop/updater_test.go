package main

// updater_test.go 覆盖自动更新的离线可测部分：
// 版本比较、Release 选择、下载校验、解包、bundle 校验、替换与回滚、版本一致性。
// 全部用本地 httptest / 临时目录，不访问外网。

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSemverCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"0.2.0", "0.1.9", 1, true},
		{"0.2.0", "0.2.0", 0, true},
		{"0.2.0", "0.2.1", -1, true},
		{"desktop-v0.3.0", "0.2.9", 1, true},
		{"v1.0", "1.0.0", 0, true},
		{"bad", "0.1.0", 0, false},
		{"", "0.1.0", 0, false},
	}
	for _, c := range cases {
		got, ok := compareSemver(c.a, c.b)
		if ok != c.ok || (ok && got != c.want) {
			t.Fatalf("compareSemver(%q,%q) = (%d,%v)，期望 (%d,%v)", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
}

func TestUpdateAssetName(t *testing.T) {
	if name, ok := updateAssetName("darwin", "arm64"); !ok || name != "chaoxing-desktop-macos-arm64.zip" {
		t.Fatalf("darwin/arm64 = %q,%v", name, ok)
	}
	if name, ok := updateAssetName("darwin", "amd64"); !ok || name != "chaoxing-desktop-macos-amd64.zip" {
		t.Fatalf("darwin/amd64 = %q,%v", name, ok)
	}
	if _, ok := updateAssetName("linux", "amd64"); ok {
		t.Fatal("linux 不应返回更新包名")
	}
}

func updateTestAssetName(t *testing.T) string {
	t.Helper()
	name, ok := updateAssetName("darwin", "arm64")
	if !ok {
		t.Fatal("测试环境缺少 darwin/arm64 资源名")
	}
	return name
}

// releasesJSON 生成 GitHub releases API 的测试响应。
func releasesJSON(t *testing.T, tag string, version string, assetName string, digest string) []byte {
	t.Helper()
	body := []map[string]any{{
		"tag_name":     tag,
		"draft":        false,
		"prerelease":   false,
		"body":         "修复若干问题",
		"html_url":     "https://example.com/release",
		"published_at": "2026-11-05T00:00:00Z",
		"assets": []map[string]any{{
			"name":                 assetName,
			"size":                 1024,
			"digest":               digest,
			"browser_download_url": "https://example.com/" + assetName,
		}},
	}}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCheckForUpdateSelectsLatestStable(t *testing.T) {
	asset := updateTestAssetName(t)
	digest := "sha256:" + strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/repos/"+updateRepoSlug+"/releases") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"tag_name": "desktop-v0.0.9", "draft": false, "assets": []any{}},
			{"tag_name": "desktop-v0.2.1", "draft": true, "assets": []any{}},
			{"tag_name": "desktop-v0.1.5", "draft": false, "prerelease": false,
				"body": "旧版本", "html_url": "https://example.com/old", "assets": []any{}},
			{"tag_name": "desktop-v0.2.0", "draft": false, "prerelease": false,
				"body": "新版本", "html_url": "https://example.com/new",
				"assets": []any{map[string]any{
					"name": asset, "size": 2048, "digest": digest,
					"browser_download_url": "https://example.com/" + asset,
				}}},
		})
	}))
	defer server.Close()

	deps := updateDeps{apiBase: server.URL, client: server.Client(), goos: "darwin", goarch: "arm64"}
	info, err := checkForUpdate(context.Background(), deps, "0.1.0")
	if err != nil {
		t.Fatalf("检查更新失败：%v", err)
	}
	if !info.Available || info.LatestVersion != "0.2.0" || info.Tag != "desktop-v0.2.0" {
		t.Fatalf("选择结果不对：%+v", info)
	}
	if info.AssetName != asset || info.AssetSize != 2048 || info.Digest != digest {
		t.Fatalf("资源信息不对：%+v", info)
	}
	if info.Notes != "新版本" || info.ReleaseURL != "https://example.com/new" {
		t.Fatalf("Release 信息不对：%+v", info)
	}
}

func TestCheckForUpdateNoReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	deps := updateDeps{apiBase: server.URL, client: server.Client(), goos: "darwin", goarch: "arm64"}
	info, err := checkForUpdate(context.Background(), deps, "0.2.0")
	if err != nil {
		t.Fatalf("没有 Release 不应报错：%v", err)
	}
	if !info.NoReleases || info.Available {
		t.Fatalf("应标记 NoReleases：%+v", info)
	}
}

func TestCheckForUpdateMissingAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"tag_name": "desktop-v0.2.0", "draft": false, "prerelease": false,
			"assets": []any{map[string]any{
				"name": "chaoxing-desktop-macos-amd64.zip", "size": 1,
				"digest":               "sha256:" + strings.Repeat("a", 64),
				"browser_download_url": "https://example.com/x.zip",
			}},
		}})
	}))
	defer server.Close()
	deps := updateDeps{apiBase: server.URL, client: server.Client(), goos: "darwin", goarch: "arm64"}
	if _, err := checkForUpdate(context.Background(), deps, "0.1.0"); !errors.Is(err, errUpdateNoAsset) {
		t.Fatalf("缺少架构资源时应返回 errUpdateNoAsset，实际 %v", err)
	}
}

func TestCheckForUpdateHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()
	deps := updateDeps{apiBase: server.URL, client: server.Client(), goos: "darwin", goarch: "arm64"}
	if _, err := checkForUpdate(context.Background(), deps, "0.1.0"); err == nil {
		t.Fatal("HTTP 500 应返回错误")
	}
}

func sha256Of(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestDownloadFileVerifiesDigest(t *testing.T) {
	payload := []byte("update-payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "14")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dst := filepath.Join(t.TempDir(), "update.zip")
	progressCalls := 0
	err := downloadFile(context.Background(), server.Client(), server.URL, dst, sha256Of(payload), int64(len(payload)), func(downloaded, total int64) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("正确校验值应下载成功：%v", err)
	}
	if progressCalls == 0 {
		t.Fatal("应回调下载进度")
	}
	if data, err := os.ReadFile(dst); err != nil || string(data) != string(payload) {
		t.Fatalf("下载内容不对：%v %q", err, data)
	}

	badDst := filepath.Join(t.TempDir(), "bad.zip")
	err = downloadFile(context.Background(), server.Client(), server.URL, badDst, "sha256:"+strings.Repeat("0", 64), 0, nil)
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("错误校验值应被拒绝，实际 %v", err)
	}
}

func makeTestZip(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractZipArchive(t *testing.T) {
	zipPath := makeTestZip(t, map[string]string{
		"超星助手.app/Contents/Info.plist":             "plist",
		"超星助手.app/Contents/MacOS/chaoxing-desktop": "bin",
	})
	dest := t.TempDir()
	if err := extractZipArchive(zipPath, dest); err != nil {
		t.Fatalf("解压失败：%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "超星助手.app", "Contents", "Info.plist")); err != nil || string(data) != "plist" {
		t.Fatalf("解压内容不对：%v %q", err, data)
	}
	if _, err := findAppBundle(dest); err != nil {
		t.Fatalf("应能找到 app bundle：%v", err)
	}
}

func TestExtractZipArchiveRejectsTraversal(t *testing.T) {
	zipPath := makeTestZip(t, map[string]string{"../evil.txt": "x"})
	if err := extractZipArchive(zipPath, t.TempDir()); err == nil {
		t.Fatal("应拒绝 ../ 路径穿越")
	}
}

func TestValidateUpdateBundle(t *testing.T) {
	appDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(appDir, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "Contents", "Info.plist"), []byte("plist"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := func(bundleID, version string, signErr error) func(string, ...string) ([]byte, error) {
		return func(name string, args ...string) ([]byte, error) {
			switch name {
			case "plutil":
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "CFBundleIdentifier") {
					return []byte(bundleID + "\n"), nil
				}
				return []byte(version + "\n"), nil
			case "codesign":
				return nil, signErr
			}
			return nil, nil
		}
	}

	if err := validateUpdateBundle(appDir, "0.2.0", runner("com.wails.chaoxing-desktop", "0.2.0", nil)); err != nil {
		t.Fatalf("合法 bundle 应通过：%v", err)
	}
	if err := validateUpdateBundle(appDir, "0.2.0", runner("com.example.evil", "0.2.0", nil)); err == nil {
		t.Fatal("错误 bundle id 应被拒绝")
	}
	if err := validateUpdateBundle(appDir, "0.2.0", runner("com.wails.chaoxing-desktop", "0.1.0", nil)); err == nil {
		t.Fatal("版本不一致应被拒绝")
	}
	if err := validateUpdateBundle(appDir, "0.2.0", runner("com.wails.chaoxing-desktop", "0.2.0", errors.New("bad signature"))); err == nil {
		t.Fatal("签名失败应被拒绝")
	}
}

func TestInstallAppBundleReplaceAndRollback(t *testing.T) {
	parent := t.TempDir()
	current := filepath.Join(parent, "超星助手.app")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(parent, "staged.app")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	backup, err := installAppBundle(current, staged)
	if err != nil {
		t.Fatalf("替换失败：%v", err)
	}
	if _, err := os.Stat(filepath.Join(current, "new.txt")); err != nil {
		t.Fatalf("新版本未就位：%v", err)
	}
	if _, err := os.Stat(filepath.Join(backup, "old.txt")); err != nil {
		t.Fatalf("旧版本备份缺失：%v", err)
	}
	_ = os.RemoveAll(backup)

	// 失败回滚：staged 不存在时，旧版本应被还原
	missingStaged := filepath.Join(parent, "missing.app")
	if _, err := installAppBundle(current, missingStaged); err == nil {
		t.Fatal("staged 不存在应失败")
	}
	if _, err := os.Stat(filepath.Join(current, "new.txt")); err != nil {
		t.Fatalf("回滚后旧/新版本缺失：%v", err)
	}
}

func TestVersionMatchesWailsConfig(t *testing.T) {
	data, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("读取 wails.json 失败：%v", err)
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("解析 wails.json 失败：%v", err)
	}
	if cfg.Info.ProductVersion != appVersion {
		t.Fatalf("wails.json productVersion=%q 与 version.go appVersion=%q 不一致", cfg.Info.ProductVersion, appVersion)
	}
}

func TestUpdateAutoCheckEnvSwitch(t *testing.T) {
	t.Setenv("CX_NO_UPDATE_CHECK", "1")
	if updateAutoCheckEnabled() {
		t.Fatal("CX_NO_UPDATE_CHECK=1 时应关闭自动检查")
	}
	t.Setenv("CX_NO_UPDATE_CHECK", "")
	if !updateAutoCheckEnabled() {
		t.Fatal("默认应开启自动检查")
	}
}

// ---- 以下为 macOS 上的完整更新流水线测试（需要 codesign；其他平台跳过） ----

func createSignedFakeApp(t *testing.T, parentDir, appName, version string) string {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("完整更新流水线测试仅支持 macOS")
	}
	if _, err := exec.LookPath("codesign"); err != nil {
		t.Skip("没有 codesign，跳过")
	}
	appDir := filepath.Join(parentDir, appName)
	macOSDir := filepath.Join(appDir, "Contents", "MacOS")
	if err := os.MkdirAll(macOSDir, 0o755); err != nil {
		t.Fatal(err)
	}
	infoPlist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.wails.chaoxing-desktop</string>
<key>CFBundleShortVersionString</key><string>` + version + `</string>
<key>CFBundleExecutable</key><string>chaoxing-desktop</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(appDir, "Contents", "Info.plist"), []byte(infoPlist), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macOSDir, "chaoxing-desktop"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("codesign", "--force", "--deep", "--sign", "-", appDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ad-hoc 签名失败：%v %s", err, out)
	}
	return appDir
}

func zipDetail(t *testing.T, srcDir, zipPath, rootName string) {
	t.Helper()
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(filepath.Dir(srcDir), path)
		if err != nil {
			return err
		}
		_ = rootName
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRunUpdatePipelineEndToEnd(t *testing.T) {
	currentParent := t.TempDir()
	currentApp := createSignedFakeApp(t, currentParent, "超星助手.app", "0.2.0")

	stageParent := t.TempDir()
	stagedApp := createSignedFakeApp(t, stageParent, "超星助手.app", "0.2.1")
	zipPath := filepath.Join(t.TempDir(), "update.zip")
	zipDetail(t, stagedApp, zipPath, "超星助手.app")
	zipBytes, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases") {
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"tag_name": "desktop-v0.2.1", "draft": false, "prerelease": false,
				"body": "test", "html_url": "https://example.com/release",
				"assets": []any{map[string]any{
					"name": updateTestAssetName(t), "size": len(zipBytes),
					"digest":               sha256Of(zipBytes),
					"browser_download_url": "http://" + r.Host + "/update.zip",
				}},
			}})
			return
		}
		_, _ = w.Write(zipBytes)
	}))
	defer server.Close()

	deps := updateDeps{apiBase: server.URL, client: server.Client(), goos: "darwin", goarch: "arm64",
		runCmd: execCommandOutput}
	info, err := checkForUpdate(context.Background(), deps, "0.2.0")
	if err != nil || !info.Available {
		t.Fatalf("检查更新失败：%v %+v", err, info)
	}
	backup, err := runUpdatePipeline(context.Background(), deps, info, currentApp, nil, func(string) error { return nil })
	if err != nil {
		t.Fatalf("更新流水线失败：%v", err)
	}
	versionOut, err := execCommandOutput("plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-",
		filepath.Join(currentApp, "Contents", "Info.plist"))
	if err != nil {
		t.Fatalf("读取更新后版本失败：%v", err)
	}
	if strings.TrimSpace(string(versionOut)) != "0.2.1" {
		t.Fatalf("更新后版本 = %q，期望 0.2.1", strings.TrimSpace(string(versionOut)))
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("旧版本备份缺失：%v", err)
	}
}

func TestRunUpdatePipelineRejectsBadDigest(t *testing.T) {
	currentParent := t.TempDir()
	currentApp := createSignedFakeApp(t, currentParent, "超星助手.app", "0.2.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases") {
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"tag_name": "desktop-v0.2.1", "draft": false, "prerelease": false,
				"assets": []any{map[string]any{
					"name": updateTestAssetName(t), "size": 4,
					"digest":               "sha256:" + strings.Repeat("0", 64),
					"browser_download_url": "http://" + r.Host + "/update.zip",
				}},
			}})
			return
		}
		_, _ = w.Write([]byte("junk"))
	}))
	defer server.Close()
	deps := updateDeps{apiBase: server.URL, client: server.Client(), goos: "darwin", goarch: "arm64",
		runCmd: execCommandOutput}
	info, err := checkForUpdate(context.Background(), deps, "0.2.0")
	if err != nil {
		t.Fatalf("检查更新失败：%v", err)
	}
	if _, err := runUpdatePipeline(context.Background(), deps, info, currentApp, nil, func(string) error { return nil }); err == nil {
		t.Fatal("sha256 不匹配时应失败")
	}
	versionOut, err := execCommandOutput("plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-",
		filepath.Join(currentApp, "Contents", "Info.plist"))
	if err != nil || strings.TrimSpace(string(versionOut)) != "0.2.0" {
		t.Fatalf("失败后当前版本应保持不变：%v %q", err, versionOut)
	}
}

func TestCleanupStaleUpdateArtifactsOnlyRemovesOurPrefixes(t *testing.T) {
	parent := t.TempDir()
	app := filepath.Join(parent, "超星助手.app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(parent, "user-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(dataDir, "config.ini")
	if err := os.WriteFile(sentinel, []byte("[ai]\nkey=keep-me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleStage := filepath.Join(parent, updateStagePrefix+"old")
	staleOld := filepath.Join(parent, updateOldBundlePrefix+"old-超星助手.app")
	if err := os.MkdirAll(staleStage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staleOld, 0o755); err != nil {
		t.Fatal(err)
	}

	cleanupStaleUpdateArtifacts(app)

	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "[ai]\nkey=keep-me\n" {
		t.Fatalf("更新清理不应碰用户数据：%v %q", err, data)
	}
	if _, err := os.Stat(staleStage); !os.IsNotExist(err) {
		t.Fatalf("更新临时目录应被清理：%v", err)
	}
	if _, err := os.Stat(staleOld); !os.IsNotExist(err) {
		t.Fatalf("旧版本备份应被清理：%v", err)
	}
}
