package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
)

// revealRequest 是 /api/reveal 的请求体。
type revealRequest struct {
	Path string `json:"path"`
}

// errNoActiveSession 表示没有找到可投递的交互式桌面会话。
var errNoActiveSession = errors.New("no active interactive session")

// handleReveal 在系统文件管理器中定位博客文章。
//
// 定位属于本机桌面操作，因此只接受回环地址请求：即使服务绑定到局域网并启用了
// Basic Auth，远程用户也无法通过该接口在宿主机上弹出窗口。路径会先按博客根目录
// 做符号链接校验，越界访问一律拒绝。
func (a *App) handleReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, `{"error":"仅支持 POST"}`, http.StatusMethodNotAllowed)
		return
	}
	if !isLoopbackRequest(r) {
		http.Error(w, `{"error":"仅允许本机调用"}`, http.StatusForbidden)
		return
	}

	var req revealRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, `{"error":"请求格式错误"}`, http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, `{"error":"缺少 path 参数"}`, http.StatusBadRequest)
		return
	}

	full, err := resolveInsideRoot(a.root, req.Path)
	if err != nil {
		if errors.Is(err, errUnsafePath) {
			http.Error(w, `{"error":"禁止访问"}`, http.StatusForbidden)
		} else {
			http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		}
		return
	}

	if err := revealFn(full); err != nil {
		log.Printf("定位文件失败 %s: %v", full, err)
		// 把失败原因回传给前端，避免用户只看到"点了没反应"。
		msg, _ := json.Marshal(map[string]string{
			"error":  "无法打开文件管理器",
			"detail": err.Error(),
		})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(msg)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "path": full})
}

// revealFn 是文件管理器定位的实际实现，测试中可替换以避免弹出系统窗口。
var revealFn = revealInFileManager

// revealInFileManager 在系统文件管理器中选中目标文件。
func revealInFileManager(fullPath string) error {
	switch runtime.GOOS {
	case "windows":
		return revealOnWindows(fullPath)
	case "darwin":
		return exec.Command("open", "-R", fullPath).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(fullPath)).Start()
	}
}

// revealOnWindows 优先在交互式桌面会话中打开资源管理器。
//
// 当本程序作为 Windows 服务运行时（NSSM / sc），进程处于 Session 0。
// Session 0 与用户桌面隔离，直接启动 explorer.exe 只会得到一个在本会话内
// 不可见的窗口，用户侧表现为"点击无反应"。因此这里先尝试把资源管理器
// 投递到真实登录的交互式会话。
//
// 关键：不能把"进程启动成功"当作"用户看到了窗口"。若跨会话投递失败，
// 而本进程又处于 Session 0，则必须如实报错，而不是静默地在不可见会话里
// 启动一个窗口——否则前端无从提示，用户只会看到按钮点了没反应。
func revealOnWindows(fullPath string) error {
	err := revealInActiveSession(fullPath)
	if err == nil {
		return nil
	}
	log.Printf("⚠️ 跨会话投递失败：%v", err)

	if runningInSessionZero() {
		// Session 0 内启动的窗口用户永远看不到，直接失败比假装成功更有用。
		return fmt.Errorf("无法在桌面会话打开（服务运行于 Session 0）：%w", err)
	}
	// 以普通桌面程序运行：直接启动即可，窗口就在当前会话。
	return launchExplorer(fullPath)
}
