//go:build windows

package main

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	modkernel32             = syscall.NewLazyDLL("kernel32.dll")
	modwtsapi32             = syscall.NewLazyDLL("wtsapi32.dll")
	modadvapi32             = syscall.NewLazyDLL("advapi32.dll")
	moduserenv              = syscall.NewLazyDLL("userenv.dll")
	procWTSGetActiveConsole = modkernel32.NewProc("WTSGetActiveConsoleSessionId")
	procWTSQueryUserToken   = modwtsapi32.NewProc("WTSQueryUserToken")
	procDuplicateTokenEx    = modadvapi32.NewProc("DuplicateTokenEx")
	procCreateProcessAsUser = modadvapi32.NewProc("CreateProcessAsUserW")
	procCreateEnvBlock      = moduserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvBlock     = moduserenv.NewProc("DestroyEnvironmentBlock")
	procOpenProcessToken    = modadvapi32.NewProc("OpenProcessToken")
	procLookupPrivilege     = modadvapi32.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivs    = modadvapi32.NewProc("AdjustTokenPrivileges")
	procGetCurrentProcess   = modkernel32.NewProc("GetCurrentProcess")
	procProcessIdToSession  = modkernel32.NewProc("ProcessIdToSessionId")
)

const (
	tokenPrimary     = 1
	securityImperson = 2
	createUnicodeEnv = 0x00000400
	startfUseShowWin = 0x00000001
	swShowNormal     = 1
)

// activeConsoleSessionID 返回当前物理控制台上登录的会话 ID；无用户登录时为 0。
func activeConsoleSessionID() uint32 {
	r, _, _ := procWTSGetActiveConsole.Call()
	return uint32(r)
}

// currentSessionID 返回本进程所在的会话 ID。
// 以 Windows 服务运行时通常为 0，而用户桌面在 1 及以上。
func currentSessionID() uint32 {
	var sid uint32
	pid := uint32(syscall.Getpid())
	if ok, _, _ := procProcessIdToSession.Call(uintptr(pid), uintptr(unsafe.Pointer(&sid))); ok == 0 {
		return 0xFFFFFFFF
	}
	return sid
}

// runningInSessionZero 判断进程是否运行在 Session 0（服务会话，无用户桌面）。
func runningInSessionZero() bool {
	return currentSessionID() == 0
}

// revealInActiveSession 把资源管理器启动到交互式桌面会话中。
//
// 以 Windows 服务（LocalSystem）运行时进程处于 Session 0，与用户桌面隔离：
// 直接启动 explorer.exe 只会得到本会话内不可见的窗口，用户侧表现为"点击无反应"。
// 这里取得当前登录用户的主令牌，在其会话的 winsta0\default 桌面上创建进程。
// 以普通桌面程序运行时 WTSQueryUserToken 会失败，由调用方退回直接启动。
func revealInActiveSession(fullPath string) error {
	sessionID := activeConsoleSessionID()
	if sessionID == 0 {
		return errNoActiveSession
	}

	// 三个权限缺一不可，且在 LocalSystem 令牌中默认都是"存在但未启用"：
	//   SeTcbPrivilege            —— WTSQueryUserToken 的前置条件
	//   SeAssignPrimaryTokenPrivilege —— CreateProcessAsUser 指定令牌所需
	//   SeIncreaseQuotaPrivilege      —— 为新进程分配配额所需
	// 只启用第一个的话，WTSQueryUserToken 会通过，但 CreateProcessAsUser 必然
	// 返回 1314 (A required privilege is not held by the client)。
	if err := enablePrivileges(
		"SeTcbPrivilege",
		"SeAssignPrimaryTokenPrivilege",
		"SeIncreaseQuotaPrivilege",
	); err != nil {
		log.Printf("⚠️ 启用服务会话权限失败：%v", err)
	}

	var userToken syscall.Token
	if ok, _, err := procWTSQueryUserToken.Call(uintptr(sessionID), uintptr(unsafe.Pointer(&userToken))); ok == 0 {
		return fmt.Errorf("WTSQueryUserToken(session=%d): %v", sessionID, err)
	}
	defer userToken.Close()

	// explorer 需要主令牌才能在新会话创建桌面窗口。
	var primaryToken syscall.Token
	if ok, _, err := procDuplicateTokenEx.Call(
		uintptr(userToken),
		uintptr(syscall.TOKEN_ALL_ACCESS),
		0,
		uintptr(securityImperson),
		uintptr(tokenPrimary),
		uintptr(unsafe.Pointer(&primaryToken)),
	); ok == 0 {
		return fmt.Errorf("DuplicateTokenEx: %v", err)
	}
	defer primaryToken.Close()

	env, _ := createEnvironmentBlock(primaryToken)
	log.Printf("定位文件：投递到会话 %d 的桌面（CreateProcessAsUser）", sessionID)
	if env != 0 {
		defer procDestroyEnvBlock.Call(env)
	}

	app, err := syscall.UTF16PtrFromString(`C:\Windows\explorer.exe`)
	if err != nil {
		return err
	}
	// explorer 解析 /select, 的方式很特殊：开关必须留在引号外，只有路径被引号包住。
	//   ✔ explorer.exe /select,"C:\dir b.html"   → 打开 dir 并选中文件
	//   ✘ explorer.exe "/select,C:\dir b.html"   → 整个串被当成一个参数，
	//                                                  开关无法识别，退回"文档"目录
	// 因此这里手工拼命令行，不能用 exec.Command（它会为含空格参数整体加引号）。
	cmdLine, err := syscall.UTF16PtrFromString(explorerCommandLine(fullPath))
	if err != nil {
		return err
	}
	desktop, err := syscall.UTF16PtrFromString(`winsta0\default`)
	if err != nil {
		return err
	}

	var si syscall.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags |= startfUseShowWin
	si.ShowWindow = swShowNormal
	si.Desktop = desktop

	var pi syscall.ProcessInformation
	var zeroDir *uint16

	if ok, _, err := procCreateProcessAsUser.Call(
		uintptr(primaryToken),
		uintptr(unsafe.Pointer(app)),
		uintptr(unsafe.Pointer(cmdLine)),
		0, 0, 0,
		uintptr(createUnicodeEnv),
		env,
		uintptr(unsafe.Pointer(zeroDir)),
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	); ok == 0 {
		return fmt.Errorf("CreateProcessAsUser: %v", err)
	}

	syscall.CloseHandle(pi.Thread)
	syscall.CloseHandle(pi.Process)
	return nil
}

// createEnvironmentBlock 为用户令牌构造环境变量块。
// 拿不到用户环境并不致命，返回 0 时资源管理器以默认环境启动。
func createEnvironmentBlock(token syscall.Token) (uintptr, error) {
	var env uintptr
	if ok, _, _ := procCreateEnvBlock.Call(uintptr(unsafe.Pointer(&env)), uintptr(token), 0); ok == 0 {
		return 0, nil
	}
	return env, nil
}

// enablePrivileges 在当前进程令牌上批量启用指定权限。
//
// 若某项权限本就不存在于令牌中（如以普通用户运行），该调用会被跳过而不报错；
// 真正的缺失会在后续 WTSQueryUserToken / CreateProcessAsUser 处暴露。
func enablePrivileges(names ...string) error {
	var token syscall.Token
	h, _, _ := procGetCurrentProcess.Call()
	if ok, _, err := procOpenProcessToken.Call(
		h,
		uintptr(syscall.TOKEN_ADJUST_PRIVILEGES|syscall.TOKEN_QUERY),
		uintptr(unsafe.Pointer(&token)),
	); ok == 0 {
		return fmt.Errorf("OpenProcessToken: %v", err)
	}
	defer token.Close()

	type luidAndAttrs struct {
		Luid struct {
			LowPart  uint32
			HighPart int32
		}
		Attributes uint32
	}
	type tokenPrivileges struct {
		PrivilegeCount uint32
		Privileges     [8]luidAndAttrs
	}

	var tp tokenPrivileges
	for _, name := range names {
		pname, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		var luid struct {
			LowPart  uint32
			HighPart int32
		}
		if ok, _, _ := procLookupPrivilege.Call(0, uintptr(unsafe.Pointer(pname)), uintptr(unsafe.Pointer(&luid))); ok == 0 {
			continue
		}
		idx := tp.PrivilegeCount
		if int(idx) >= len(tp.Privileges) {
			break
		}
		tp.Privileges[idx].Luid = luid
		tp.Privileges[idx].Attributes = 0x00000002 // SE_PRIVILEGE_ENABLED
		tp.PrivilegeCount++
	}
	if tp.PrivilegeCount == 0 {
		return errors.New("没有任何可启用的权限")
	}

	if ok, _, err := procAdjustTokenPrivs.Call(
		uintptr(token), 0, uintptr(unsafe.Pointer(&tp)), 0, 0, 0,
	); ok == 0 {
		return fmt.Errorf("AdjustTokenPrivileges: %v", err)
	}
	return nil
}

// explorerCommandLine 拼出 explorer.exe 的完整命令行。
//
// /select, 的引号规则很容易踩坑：开关必须留在引号外，只有路径加引号。
// 写成 "/select,C:\dir\a b.html" 时整个串被当作一个参数，explorer 认不出开关，
// 会退回默认的"文档"目录——表现为"点了但打开的不是目标位置"。
func explorerCommandLine(fullPath string) string {
	return `C:\Windows\explorer.exe /select,"` + fullPath + `"`
}

// explorerCommandLineDir 打开目录（不选中文件）时的命令行。
func explorerCommandLineDir(dir string) string {
	return `C:\Windows\explorer.exe "` + dir + `"`
}

// launchExplorer 在当前会话启动资源管理器并选中文件；失败时退回打开所在目录。
//
// 用 SysProcAttr.CmdLine 原样指定命令行，因为 exec.Command 会把含空格的参数
// 整体加引号，使 "/select,路径" 变成一个参数、开关失效。
func launchExplorer(fullPath string) error {
	cmd := exec.Command(`C:\Windows\explorer.exe`)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: explorerCommandLine(fullPath)}
	if err := cmd.Start(); err == nil {
		return nil
	}

	dir := filepath.Dir(fullPath)
	fallback := exec.Command(`C:\Windows\explorer.exe`)
	fallback.SysProcAttr = &syscall.SysProcAttr{CmdLine: explorerCommandLineDir(dir)}
	return fallback.Start()
}
