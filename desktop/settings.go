package main

// settings.go 负责桌面端自己的设置文件 ~/.chaoxing/desktop.json。
//
// 该文件与引擎的 config.ini 分开：这里只保存桌面应用自己的配置
// （引擎仓库路径、Python 路径），可为空。写文件使用 0600 权限。

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// desktopSettings 是 ~/.chaoxing/desktop.json 的结构。
type desktopSettings struct {
	RepoPath   string `json:"repo_path"`
	PythonPath string `json:"python_path"`
}

// userHomeDir 返回当前用户 home 目录（测试可通过注入 ProbeOptions.HomeDir 绕开）。
func userHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("无法读取用户目录：%v", err)
	}
	return home, nil
}

// dataDirForHome 返回引擎数据目录：优先 CX_DATA_HOME，否则 <home>/.chaoxing。
func dataDirForHome(home string) string {
	if v := strings.TrimSpace(os.Getenv("CX_DATA_HOME")); v != "" {
		return v
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".chaoxing")
}

// settingsPathForDataDir 返回 deskop.json 的完整路径。
func settingsPathForDataDir(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "desktop.json")
}

// loadDesktopSettings 读取设置文件；文件不存在返回零值而不是错误。
func loadDesktopSettings(path string) (desktopSettings, error) {
	var s desktopSettings
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return s, fmt.Errorf("读取桌面设置失败：%v", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return s, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return desktopSettings{}, fmt.Errorf("桌面设置文件格式有误（%s）：请删除后重试", path)
	}
	return s, nil
}

// saveDesktopSettings 原子地写入设置文件（先写临时文件再 rename）。
func saveDesktopSettings(path string, s desktopSettings) error {
	if path == "" {
		return errors.New("设置文件路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建设置目录失败：%v", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化桌面设置失败：%v", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("写入桌面设置失败：%v", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存桌面设置失败：%v", err)
	}
	return nil
}
