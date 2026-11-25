@echo off
rem 以管理员身份运行同目录下的 重启服务.ps1
rem 双击本文件会触发 UAC 提权，然后执行 PowerShell 脚本。

setlocal
cd /d "%~dp0"

net session >nul 2>&1
if %errorLevel% neq 0 (
    echo Requesting administrator privileges...
    powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0重启服务.ps1"
