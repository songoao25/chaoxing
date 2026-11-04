package main

// engine.go 定位引擎仓库（含 main.py + api/ 的 Python 项目）和可用的 Python。
//
// 探测顺序（契约与任务要求）：
//  1. ~/.chaoxing/desktop.json 里的 repo_path；
//  2. 从可执行文件所在目录逐级向上、从当前工作目录逐级向上，找含 main.py+api/ 的目录；
//  3. <home>/code/chaoxing。
//
// Python 候选：设置值 python_path → <repo>/.venv/bin/python3 → <repo>/.venv/bin/python
// → PATH 上的 python3。每个候选执行 `-c "import sys;print(sys.version)"`，
// 要求版本 ≥ 3.13，成功者记录其版本字符串（如 "3.14.7"）。
//
// 所有探测细节通过 ProbeOptions 注入，测试用临时目录伪造引擎，绝不写真实 ~/.chaoxing。

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const minPythonMinor = 13 // 要求 Python >= 3.13

// probeOptions 控制探测行为；零值字段使用默认实现。
type probeOptions struct {
	// SettingsPath 为 desktop.json 路径；空则按 HomeDir/CX_DATA_HOME 推导。
	SettingsPath string
	// DataDir 为 ~/.chaoxing；空则按 HomeDir/CX_DATA_HOME 推导。
	DataDir string
	HomeDir string
	// ExePath 为当前可执行文件路径；空则 os.Executable()。
	ExePath string
	// Cwd 为当前工作目录；空则 os.Getwd()。
	Cwd string
	// RepoCandidates 为额外候选（测试用），排在设置值之后、逐级向上之前。
	RepoCandidates []string
	// LookPath 默认 exec.LookPath。
	LookPath func(string) (string, error)
	// RunPythonVersion 执行 python -c 并返回版本输出；默认真实执行。
	RunPythonVersion func(python string) (string, error)
	// Stat 默认 os.Stat。
	Stat func(string) (os.FileInfo, error)
}

// engineFinding 是探测结果，对外暴露为 EngineStatus（§5）。
type engineFinding struct {
	Repo      string
	PythonExe string // Python 可执行文件绝对路径（启动桥用）
	Python    string // 版本字符串，如 "3.14.7"
	Ready     bool
	Message   string
}

// status 转成对前端暴露的结构。
func (e engineFinding) status() EngineStatus {
	return EngineStatus{Repo: e.Repo, Python: e.Python, Ready: e.Ready, Message: e.Message}
}

func (o probeOptions) stat() func(string) (os.FileInfo, error) {
	if o.Stat != nil {
		return o.Stat
	}
	return os.Stat
}

func (o probeOptions) lookPath() func(string) (string, error) {
	if o.LookPath != nil {
		return o.LookPath
	}
	return exec.LookPath
}

func (o probeOptions) pythonVersion() func(string) (string, error) {
	if o.RunPythonVersion != nil {
		return o.RunPythonVersion
	}
	return runPythonVersion
}

func (o probeOptions) exePath() string {
	if o.ExePath != "" {
		return o.ExePath
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err2 := filepath.EvalSymlinks(exe); err2 == nil {
		return resolved
	}
	return exe
}

func (o probeOptions) cwd() string {
	if o.Cwd != "" {
		return o.Cwd
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func (o probeOptions) home() string {
	if o.HomeDir != "" {
		return o.HomeDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func (o probeOptions) dataDir() string {
	if o.DataDir != "" {
		return o.DataDir
	}
	return dataDirForHome(o.home())
}

func (o probeOptions) settingsPath() string {
	if o.SettingsPath != "" {
		return o.SettingsPath
	}
	return settingsPathForDataDir(o.dataDir())
}

// runPythonVersion 真实执行 python -c 校验版本输出。
func runPythonVersion(python string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, python, "-c", "import sys;print(sys.version)").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", firstLine(msg))
	}
	return strings.TrimSpace(string(out)), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// parsePythonVersion 从 `sys.version` 输出中解析 major/minor/patch。
func parsePythonVersion(output string) (major, minor, patch int, version string, err error) {
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return 0, 0, 0, "", fmt.Errorf("Python 版本输出为空")
	}
	token := fields[0]
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return 0, 0, 0, "", fmt.Errorf("无法识别的 Python 版本：%s", token)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("无法识别的 Python 版本：%s", token)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("无法识别的 Python 版本：%s", token)
	}
	if len(parts) == 3 {
		num := parts[2]
		for i, r := range num {
			if r < '0' || r > '9' {
				num = num[:i]
				break
			}
		}
		if num != "" {
			patch, _ = strconv.Atoi(num)
		}
	}
	if major <= 0 || minor < 0 {
		return 0, 0, 0, "", fmt.Errorf("无法识别的 Python 版本：%s", token)
	}
	return major, minor, patch, token, nil
}

// versionSupported 判断版本是否 ≥ 3.13。
func versionSupported(major, minor int) bool {
	return major > 3 || (major == 3 && minor >= minPythonMinor)
}

// isEngineRepo 判断目录是否是引擎仓库根（含 main.py 文件与 api/ 目录）。
func isEngineRepo(dir string, stat func(string) (os.FileInfo, error)) bool {
	if dir == "" {
		return false
	}
	info, err := stat(filepath.Join(dir, "main.py"))
	if err != nil || info.IsDir() {
		return false
	}
	apiInfo, err := stat(filepath.Join(dir, "api"))
	if err != nil || !apiInfo.IsDir() {
		return false
	}
	return true
}

// walkUpCandidates 从 start 目录逐级向上，返回所有祖先目录（含 start）。
func walkUpCandidates(start string, stat func(string) (os.FileInfo, error)) []string {
	var out []string
	dir := filepath.Clean(start)
	if dir == "." || dir == "" {
		return out
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	for {
		out = append(out, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return out
}

// detectEngine 执行完整探测，返回中文状态。
func detectEngine(opts probeOptions) engineFinding {
	stat := opts.stat()
	home := opts.home()
	settings, settingsErr := loadDesktopSettings(opts.settingsPath())

	// ---- 1. 定位引擎仓库 ----
	var repoCandidates []string
	seen := map[string]bool{}
	addCandidate := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if !filepath.IsAbs(p) && home != "" {
			p = filepath.Join(home, p)
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		p = filepath.Clean(p)
		if seen[p] {
			return
		}
		seen[p] = true
		repoCandidates = append(repoCandidates, p)
	}

	if settingsErr == nil {
		addCandidate(settings.RepoPath)
	} else {
		// 设置文件损坏不应阻断探测，只是忽略其中的值。
		settings = desktopSettings{}
	}
	for _, p := range opts.RepoCandidates {
		addCandidate(p)
	}
	if exe := opts.exePath(); exe != "" {
		for _, dir := range walkUpCandidates(filepath.Dir(exe), stat) {
			addCandidate(dir)
		}
	}
	if wd := opts.cwd(); wd != "" {
		for _, dir := range walkUpCandidates(wd, stat) {
			addCandidate(dir)
		}
	}
	if home != "" {
		addCandidate(filepath.Join(home, "code", "chaoxing"))
	}

	repo := ""
	for _, candidate := range repoCandidates {
		if isEngineRepo(candidate, stat) {
			repo = candidate
			break
		}
	}
	if repo == "" {
		return engineFinding{
			Message: "未找到引擎目录：请确认超星助手引擎（含 main.py 与 api/）已下载到本机，或在设置中选择引擎目录。",
		}
	}

	// ---- 2. 定位 Python（≥3.13）----
	var pythonCandidates []string
	pythonSeen := map[string]bool{}
	addPython := func(p string) {
		if p == "" || pythonSeen[p] {
			return
		}
		pythonSeen[p] = true
		pythonCandidates = append(pythonCandidates, p)
	}
	if settingsErr == nil && strings.TrimSpace(settings.PythonPath) != "" {
		addPython(strings.TrimSpace(settings.PythonPath))
	}
	addPython(filepath.Join(repo, ".venv", "bin", "python3"))
	addPython(filepath.Join(repo, ".venv", "bin", "python"))
	if p, err := opts.lookPath()("python3"); err == nil && p != "" {
		addPython(p)
	}

	runVersion := opts.pythonVersion()
	var versionNote string
	for _, python := range pythonCandidates {
		info, err := stat(python)
		if err != nil || info.IsDir() {
			continue
		}
		out, err := runVersion(python)
		if err != nil {
			if versionNote == "" {
				versionNote = fmt.Sprintf("运行 %s 失败：%s", filepath.Base(python), err.Error())
			}
			continue
		}
		major, minor, _, token, err := parsePythonVersion(out)
		if err != nil {
			if versionNote == "" {
				versionNote = fmt.Sprintf("无法识别 %s 的版本", filepath.Base(python))
			}
			continue
		}
		if !versionSupported(major, minor) {
			if versionNote == "" {
				versionNote = fmt.Sprintf("找到的 Python 版本为 %s（低于 3.%d）", token, minPythonMinor)
			}
			continue
		}
		return engineFinding{
			Repo:      repo,
			PythonExe: python,
			Python:    token,
			Ready:     true,
			Message:   fmt.Sprintf("引擎就绪（Python %s）", token),
		}
	}

	msg := "已找到引擎目录，但未找到可用的 Python（要求 ≥ 3.13）：请在引擎目录创建 .venv 或配置 PATH，也可在设置中指定 Python 路径。"
	if versionNote != "" {
		msg = "已找到引擎目录，但 Python 不可用：" + versionNote + "。请安装或修复 Python ≥ 3.13 后重试。"
	}
	return engineFinding{Repo: repo, Message: msg}
}

// detectEngineDefault 用默认参数执行探测。
func detectEngineDefault() engineFinding {
	return detectEngine(probeOptions{})
}
