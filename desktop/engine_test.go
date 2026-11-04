package main

// engine_test.go 覆盖引擎/仓库/Python 探测逻辑。
// 所有测试都使用 t.TempDir() 伪造目录，绝不读写真实 ~/.chaoxing。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("创建目录 %s 失败：%v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写文件 %s 失败：%v", path, err)
	}
}

// makeFakeRepo 创建一个最小的假引擎仓库（main.py + api/）。
func makeFakeRepo(t *testing.T, repo string) {
	t.Helper()
	mustMkdir(t, repo)
	mustWriteFile(t, filepath.Join(repo, "main.py"), "print('fake engine')\n")
	mustMkdir(t, filepath.Join(repo, "api"))
	mustWriteFile(t, filepath.Join(repo, "api", "__init__.py"), "")
}

// fakePython 在仓库里放一个假 Python 可执行文件。
func fakePython(t *testing.T, repo, name string) string {
	t.Helper()
	path := filepath.Join(repo, ".venv", "bin", name)
	mustWriteFile(t, path, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod 失败：%v", err)
	}
	return path
}

// versionByPython 构造按路径返回版本的注入函数。
func versionByPython(versions map[string]string) func(string) (string, error) {
	return func(python string) (string, error) {
		if v, ok := versions[python]; ok {
			return v, nil
		}
		return "3.11.0 (fallback)\n", nil
	}
}

func noLookPath(string) (string, error) { return "", os.ErrNotExist }

func TestParsePythonVersion(t *testing.T) {
	cases := []struct {
		in          string
		wantMajor   int
		wantMinor   int
		wantPatch   int
		wantVersion string
		wantErr     bool
	}{
		{"3.14.7 (main, Oct 27 2026, 10:00:00) [Clang 17.0.0]", 3, 14, 7, "3.14.7", false},
		{"3.13.0\n", 3, 13, 0, "3.13.0", false},
		{"3.13.2rc1 (main)", 3, 13, 2, "3.13.2rc1", false},
		{"", 0, 0, 0, "", true},
		{"Python 3.14.7", 0, 0, 0, "", true},
		{"3", 0, 0, 0, "", true},
	}
	for _, c := range cases {
		major, minor, patch, version, err := parsePythonVersion(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parsePythonVersion(%q) 期望报错，实际成功", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePythonVersion(%q) 报错：%v", c.in, err)
			continue
		}
		if major != c.wantMajor || minor != c.wantMinor || patch != c.wantPatch || version != c.wantVersion {
			t.Errorf("parsePythonVersion(%q) = (%d,%d,%d,%q)，期望 (%d,%d,%d,%q)",
				c.in, major, minor, patch, version, c.wantMajor, c.wantMinor, c.wantPatch, c.wantVersion)
		}
	}
}

func TestVersionSupported(t *testing.T) {
	if !versionSupported(3, 13) {
		t.Error("Python 3.13 应被接受")
	}
	if !versionSupported(3, 14) || !versionSupported(4, 0) {
		t.Error("Python 3.14/4.0 应被接受")
	}
	if versionSupported(3, 12) || versionSupported(2, 7) {
		t.Error("Python 3.12/2.7 应被拒绝")
	}
}

func TestDetectEngineFromSettings(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "engine")
	makeFakeRepo(t, repo)
	venvPython := fakePython(t, repo, "python3")

	dataDir := filepath.Join(tmp, ".chaoxing")
	mustMkdir(t, dataDir)
	settingsPath := filepath.Join(dataDir, "desktop.json")
	if err := saveDesktopSettings(settingsPath, desktopSettings{RepoPath: repo}); err != nil {
		t.Fatalf("保存设置失败：%v", err)
	}

	opts := probeOptions{
		SettingsPath:     settingsPath,
		DataDir:          dataDir,
		HomeDir:          tmp,
		ExePath:          filepath.Join(tmp, "elsewhere", "bin", "chaoxing"),
		Cwd:              filepath.Join(tmp, "elsewhere"),
		LookPath:         noLookPath,
		RunPythonVersion: versionByPython(map[string]string{venvPython: "3.14.7 (main)\n"}),
	}
	got := detectEngine(opts)
	if !got.Ready {
		t.Fatalf("期望引擎就绪，实际：%+v", got)
	}
	if got.Repo != repo {
		t.Errorf("Repo = %q，期望 %q", got.Repo, repo)
	}
	if got.Python != "3.14.7" {
		t.Errorf("Python 版本 = %q，期望 3.14.7", got.Python)
	}
	if got.PythonExe != venvPython {
		t.Errorf("PythonExe = %q，期望 %q", got.PythonExe, venvPython)
	}
	if !strings.Contains(got.Message, "就绪") {
		t.Errorf("Message 应为中文就绪说明，实际 %q", got.Message)
	}
}

func TestDetectEngineWalkUpFromExecutable(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "code", "chaoxing")
	makeFakeRepo(t, repo)
	venvPython := fakePython(t, repo, "python") // 只有 python，没有 python3

	opts := probeOptions{
		SettingsPath:     filepath.Join(tmp, "data", "desktop.json"),
		DataDir:          filepath.Join(tmp, "data"),
		HomeDir:          filepath.Join(tmp, "home"),
		ExePath:          filepath.Join(repo, "build", "bin", "chaoxing-desktop.app", "Contents", "MacOS", "chaoxing-desktop"),
		Cwd:              filepath.Join(tmp, "neutral"),
		LookPath:         noLookPath,
		RunPythonVersion: versionByPython(map[string]string{venvPython: "3.13.2 (main)\n"}),
	}
	got := detectEngine(opts)
	if !got.Ready {
		t.Fatalf("期望从可执行文件路径向上找到引擎，实际：%+v", got)
	}
	if got.Repo != repo {
		t.Errorf("Repo = %q，期望 %q", got.Repo, repo)
	}
	if got.Python != "3.13.2" {
		t.Errorf("Python 版本 = %q，期望 3.13.2", got.Python)
	}
}

func TestDetectEngineWalkUpFromCwd(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "work", "chaoxing")
	makeFakeRepo(t, repo)
	venvPython := fakePython(t, repo, "python3")

	opts := probeOptions{
		SettingsPath:     filepath.Join(tmp, "data", "desktop.json"),
		DataDir:          filepath.Join(tmp, "data"),
		HomeDir:          filepath.Join(tmp, "home"),
		ExePath:          filepath.Join(tmp, "bin", "app"),
		Cwd:              filepath.Join(repo, "tools", "sub"),
		LookPath:         noLookPath,
		RunPythonVersion: versionByPython(map[string]string{venvPython: "3.14.0 (main)\n"}),
	}
	got := detectEngine(opts)
	if !got.Ready || got.Repo != repo {
		t.Fatalf("期望从 cwd 向上找到引擎，实际：%+v", got)
	}
}

func TestDetectEngineHomeFallback(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	repo := filepath.Join(home, "code", "chaoxing")
	makeFakeRepo(t, repo)
	venvPython := fakePython(t, repo, "python3")

	opts := probeOptions{
		SettingsPath:     filepath.Join(tmp, "data", "desktop.json"),
		DataDir:          filepath.Join(tmp, "data"),
		HomeDir:          home,
		ExePath:          filepath.Join(tmp, "bin", "app"),
		Cwd:              filepath.Join(tmp, "neutral"),
		LookPath:         noLookPath,
		RunPythonVersion: versionByPython(map[string]string{venvPython: "3.14.1 (main)\n"}),
	}
	got := detectEngine(opts)
	if !got.Ready || got.Repo != repo {
		t.Fatalf("期望回退到 ~/code/chaoxing，实际：%+v", got)
	}
}

func TestDetectEngineRejectsOldPython(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "engine")
	makeFakeRepo(t, repo)
	venvPython := fakePython(t, repo, "python3")

	opts := probeOptions{
		SettingsPath:     filepath.Join(tmp, "data", "desktop.json"),
		DataDir:          filepath.Join(tmp, "data"),
		HomeDir:          filepath.Join(tmp, "home"),
		ExePath:          filepath.Join(tmp, "bin", "app"),
		Cwd:              filepath.Join(tmp, "neutral"),
		RepoCandidates:   []string{repo},
		LookPath:         noLookPath,
		RunPythonVersion: versionByPython(map[string]string{venvPython: "3.12.9 (main)\n"}),
	}
	got := detectEngine(opts)
	if got.Ready {
		t.Fatalf("Python 3.12 不应就绪，实际：%+v", got)
	}
	if got.Repo != repo {
		t.Errorf("Repo = %q，期望仍然报告已找到的仓库 %q", got.Repo, repo)
	}
	if !strings.Contains(got.Message, "3.13") {
		t.Errorf("中文错误信息应提示 3.13 要求，实际 %q", got.Message)
	}
}

func TestDetectEnginePythonCandidateOrder(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "engine")
	makeFakeRepo(t, repo)
	python3 := fakePython(t, repo, "python3")
	pythonFallback := fakePython(t, repo, "python")

	opts := probeOptions{
		SettingsPath:   filepath.Join(tmp, "data", "desktop.json"),
		DataDir:        filepath.Join(tmp, "data"),
		HomeDir:        filepath.Join(tmp, "home"),
		ExePath:        filepath.Join(tmp, "bin", "app"),
		Cwd:            filepath.Join(tmp, "neutral"),
		RepoCandidates: []string{repo},
		LookPath:       noLookPath,
		RunPythonVersion: versionByPython(map[string]string{
			python3:        "3.13.1 (main)\n",
			pythonFallback: "3.14.0 (main)\n",
		}),
	}
	got := detectEngine(opts)
	if !got.Ready || got.PythonExe != python3 {
		t.Fatalf("应优先选择 .venv/bin/python3，实际：%+v", got)
	}
}

func TestDetectEngineSettingsPythonWins(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "engine")
	makeFakeRepo(t, repo)
	venvPython := fakePython(t, repo, "python3")

	custom := filepath.Join(tmp, "custom-python", "python3")
	mustWriteFile(t, custom, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(custom, 0o755); err != nil {
		t.Fatalf("chmod 失败：%v", err)
	}

	dataDir := filepath.Join(tmp, ".chaoxing")
	mustMkdir(t, dataDir)
	settingsPath := filepath.Join(dataDir, "desktop.json")
	if err := saveDesktopSettings(settingsPath, desktopSettings{RepoPath: repo, PythonPath: custom}); err != nil {
		t.Fatalf("保存设置失败：%v", err)
	}

	opts := probeOptions{
		SettingsPath: settingsPath,
		DataDir:      dataDir,
		HomeDir:      tmp,
		ExePath:      filepath.Join(tmp, "bin", "app"),
		Cwd:          filepath.Join(tmp, "neutral"),
		LookPath:     noLookPath,
		RunPythonVersion: versionByPython(map[string]string{
			custom:     "3.14.2 (main)\n",
			venvPython: "3.13.0 (main)\n",
		}),
	}
	got := detectEngine(opts)
	if !got.Ready || got.PythonExe != custom || got.Python != "3.14.2" {
		t.Fatalf("应优先使用设置里指定的 Python，实际：%+v", got)
	}
}

func TestDetectEngineNothingFound(t *testing.T) {
	tmp := t.TempDir()
	opts := probeOptions{
		SettingsPath:     filepath.Join(tmp, "data", "desktop.json"),
		DataDir:          filepath.Join(tmp, "data"),
		HomeDir:          filepath.Join(tmp, "home"),
		ExePath:          filepath.Join(tmp, "bin", "app"),
		Cwd:              filepath.Join(tmp, "neutral"),
		LookPath:         noLookPath,
		RunPythonVersion: func(string) (string, error) { return "", os.ErrNotExist },
	}
	got := detectEngine(opts)
	if got.Ready {
		t.Fatalf("无引擎时不应就绪：%+v", got)
	}
	if got.Repo != "" || got.Python != "" {
		t.Errorf("无引擎时 Repo/Python 应为空，实际：%+v", got)
	}
	if !strings.Contains(got.Message, "未找到引擎目录") {
		t.Errorf("应有中文提示，实际 %q", got.Message)
	}
}

func TestDetectEngineSettingsRepoInvalidFallsBack(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	validRepo := filepath.Join(home, "code", "chaoxing")
	makeFakeRepo(t, validRepo)
	venvPython := fakePython(t, validRepo, "python3")

	badRepo := filepath.Join(tmp, "not-a-repo")
	mustMkdir(t, badRepo)

	dataDir := filepath.Join(tmp, ".chaoxing")
	mustMkdir(t, dataDir)
	settingsPath := filepath.Join(dataDir, "desktop.json")
	if err := saveDesktopSettings(settingsPath, desktopSettings{RepoPath: badRepo}); err != nil {
		t.Fatalf("保存设置失败：%v", err)
	}

	opts := probeOptions{
		SettingsPath:     settingsPath,
		DataDir:          dataDir,
		HomeDir:          home,
		ExePath:          filepath.Join(tmp, "bin", "app"),
		Cwd:              filepath.Join(tmp, "neutral"),
		LookPath:         noLookPath,
		RunPythonVersion: versionByPython(map[string]string{venvPython: "3.14.3 (main)\n"}),
	}
	got := detectEngine(opts)
	if !got.Ready || got.Repo != validRepo {
		t.Fatalf("设置里的无效仓库应被跳过并回退到 home 候选，实际：%+v", got)
	}
}

func TestDesktopSettingsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".chaoxing", "desktop.json")
	want := desktopSettings{RepoPath: "/tmp/engine", PythonPath: "/tmp/python3"}
	if err := saveDesktopSettings(path, want); err != nil {
		t.Fatalf("保存失败：%v", err)
	}
	got, err := loadDesktopSettings(path)
	if err != nil {
		t.Fatalf("读取失败：%v", err)
	}
	if got != want {
		t.Errorf("往返结果 = %+v，期望 %+v", got, want)
	}
	// 文件不存在时返回零值且不报错
	missing, err := loadDesktopSettings(filepath.Join(tmp, "nope", "desktop.json"))
	if err != nil || missing != (desktopSettings{}) {
		t.Errorf("缺失文件应返回零值，实际 %+v, err=%v", missing, err)
	}
	// 损坏文件返回中文错误
	badPath := filepath.Join(tmp, "bad.json")
	mustWriteFile(t, badPath, "{not json")
	if _, err := loadDesktopSettings(badPath); err == nil {
		t.Error("损坏的设置文件应返回错误")
	}
}
