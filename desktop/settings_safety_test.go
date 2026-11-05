package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDesktopSettingsBacksUpCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop.json")
	if err := os.WriteFile(path, []byte(`{"repo_path": not-json`), 0o600); err != nil {
		t.Fatalf("写入损坏设置失败：%v", err)
	}
	if _, err := loadDesktopSettings(path); err == nil {
		t.Fatal("损坏设置应返回错误")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败：%v", err)
	}
	foundBackup := false
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".corrupt-") {
			foundBackup = true
			break
		}
	}
	if !foundBackup {
		t.Fatal("损坏时应生成 .corrupt-* 备份")
	}
	original, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(original), "not-json") {
		t.Fatalf("原文件不应被改动：%v %q", err, original)
	}
}

func TestSaveDesktopSettingsAtomicKeepsPreviousData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop.json")
	first := desktopSettings{RepoPath: "/tmp/repo", OnboardingDone: true}
	if err := saveDesktopSettings(path, first); err != nil {
		t.Fatalf("首次保存失败：%v", err)
	}
	second := desktopSettings{RepoPath: "/tmp/repo", PythonPath: "/tmp/python", OnboardingDone: true}
	if err := saveDesktopSettings(path, second); err != nil {
		t.Fatalf("二次保存失败：%v", err)
	}
	loaded, err := loadDesktopSettings(path)
	if err != nil {
		t.Fatalf("读取设置失败：%v", err)
	}
	if loaded.RepoPath != "/tmp/repo" || loaded.PythonPath != "/tmp/python" || !loaded.OnboardingDone {
		t.Fatalf("设置字段丢失：%+v", loaded)
	}
}
