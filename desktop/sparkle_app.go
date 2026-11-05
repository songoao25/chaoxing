//go:build !cli

package main

import (
	"time"

	"github.com/songoao25/chaoxing/desktop/internal/sparkle"
)

// SparkleCheckForUpdates 由设置页「检查更新」调用：交给 Sparkle 展示检查/更新 UI。
func (a *App) SparkleCheckForUpdates() error {
	return sparkle.CheckForUpdates()
}

// initSparkle 在启动后初始化 Sparkle（失败只记录，不阻塞主界面）。
func (a *App) initSparkle() {
	go func() {
		time.Sleep(1500 * time.Millisecond)
		if err := sparkle.Start(); err != nil {
			a.emit("update:error", map[string]string{"message": "自动更新组件初始化失败：" + err.Error()})
		}
	}()
}
