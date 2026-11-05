package main

// onboarding.go 是「新手教程」的持久化状态：
// 首次启动展示分步引导，完成或跳过后写入 ~/.chaoxing/desktop.json 的 onboarding_done。
// 教程内容本身在前端（app.js），这里只负责“要不要显示”和“记住已完成”。

import (
	"errors"
	"fmt"
)

// GetOnboardingState 返回是否需要展示新手教程。
func (a *App) GetOnboardingState() OnboardingState {
	if a.mock {
		return OnboardingState{Needs: false}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return OnboardingState{Needs: !a.desktop.OnboardingDone}
}

// CompleteOnboarding 记录用户已完成或跳过了新手教程（设置里仍可重新打开）。
func (a *App) CompleteOnboarding() error {
	if a.mock {
		return nil
	}
	a.mu.Lock()
	a.desktop.OnboardingDone = true
	desktop := a.desktop
	settingsPath := a.settingsPath
	a.mu.Unlock()
	if settingsPath == "" {
		return errors.New("无法确定设置文件位置，教程状态未保存")
	}
	if err := saveDesktopSettings(settingsPath, desktop); err != nil {
		return fmt.Errorf("保存教程状态失败：%v", err)
	}
	return nil
}
