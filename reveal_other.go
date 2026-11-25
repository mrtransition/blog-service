//go:build !windows

package main

// revealInActiveSession 在非 Windows 平台没有会话隔离问题，始终返回不可用，
// 由调用方退回直接启动文件管理器。
func revealInActiveSession(fullPath string) error {
	return errNoActiveSession
}
