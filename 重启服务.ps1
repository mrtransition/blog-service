# 重启 blog-server 服务（NSSM 托管）并配置日志输出
#
# 用法：右键本文件 → "使用 PowerShell 运行"
#   或在管理员 PowerShell 中执行： .\重启服务.ps1
#
# 关于定位功能：
#   NSSM 以 LocalSystem 运行该服务，进程处于 Session 0，与用户桌面（Session 1）隔离。
#   直接把 explorer.exe 起在本会话里，窗口用户永远看不到，表现为"点击无反应"。
#   程序会用 WTSQueryUserToken + CreateProcessAsUser 把资源管理器投递到桌面会话，
#   这需要 SeTcbPrivilege / SeAssignPrimaryTokenPrivilege / SeIncreaseQuotaPrivilege
#   三项权限，程序启动时会自行启用。
#
# 本脚本额外把服务的 stdout/stderr 落到日志文件，便于排查。

$ErrorActionPreference = 'Stop'
$service  = 'blogserver'
$baseDir  = 'D:\ProgramFiles\blog-server'
$exe      = Join-Path $baseDir 'blog-server.exe'
$logDir   = Join-Path $baseDir 'logs'
$nssm     = 'D:\ProgramFiles\nssm-2.24\win64\nssm.exe'

Write-Host '============================================' -ForegroundColor Cyan
Write-Host '  重启 blog-server 服务（NSSM）' -ForegroundColor Cyan
Write-Host '============================================' -ForegroundColor Cyan
Write-Host ''

# --- 管理员检查 ---
$identity  = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host '[错误] 需要管理员权限。' -ForegroundColor Red
    Write-Host '请右键本文件选择“使用 PowerShell 运行”，或在管理员终端中执行。' -ForegroundColor Yellow
    Write-Host ''
    Read-Host '按回车退出'
    exit 1
}

if (-not (Test-Path $nssm)) {
    Write-Host "[错误] 未找到 nssm: $nssm" -ForegroundColor Red
    Read-Host '按回车退出'
    exit 1
}
if (-not (Test-Path $exe)) {
    Write-Host "[错误] 未找到程序: $exe" -ForegroundColor Red
    Read-Host '按回车退出'
    exit 1
}

# --- 日志目录 ---
if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}

Write-Host '[0/4] 配置日志输出...' -ForegroundColor Green
& $nssm set $service AppStdout  (Join-Path $logDir 'stdout.log') | Out-Null
& $nssm set $service AppStderr  (Join-Path $logDir 'stderr.log') | Out-Null
& $nssm set $service AppRotateFiles 1 | Out-Null
& $nssm set $service AppRotateOnline 1 | Out-Null
& $nssm set $service AppRotateBytes 10485760 | Out-Null
Write-Host "      日志: $logDir" -ForegroundColor Gray

# --- 确认运行账户 ---
$objectName = (& $nssm get $service ObjectName 2>$null | Out-String).Trim()
Write-Host "      ObjectName: $objectName" -ForegroundColor Gray
if ($objectName -notmatch 'LocalSystem') {
    Write-Host '      [警告] 非 LocalSystem 账户将无法跨会话投递资源管理器。' -ForegroundColor Yellow
}

Write-Host ''
Write-Host '[1/4] 停止服务...' -ForegroundColor Green
$svc = Get-Service -Name $service -ErrorAction SilentlyContinue
if (-not $svc) {
    Write-Host "[错误] 未找到服务 $service" -ForegroundColor Red
    Read-Host '按回车退出'
    exit 1
}
if ($svc.Status -ne 'Stopped') {
    Stop-Service -Name $service -Force
    $svc.WaitForStatus('Stopped', '00:01:00')
}
Write-Host "      $((Get-Service $service).Status)" -ForegroundColor Gray
Start-Sleep -Seconds 1

# 若存在待替换的新程序，在服务停止（文件解锁）后自动换上
$pending = Join-Path $baseDir 'blog-server.new.exe'
if (Test-Path $pending) {
    Write-Host '[1.5/4] 替换程序文件...' -ForegroundColor Green
    $backup = Join-Path $baseDir ('blog-server.exe.bak_' + (Get-Date -Format 'yyyyMMdd_HHmmss'))
    try {
        if (Test-Path $exe) { Copy-Item $exe $backup -Force }
        Copy-Item $pending $exe -Force
        Remove-Item $pending -Force
        Write-Host "      已替换（备份: $(Split-Path -Leaf $backup)）" -ForegroundColor Gray
        $ver = (Get-Item $exe).LastWriteTime
        Write-Host "      新程序时间: $ver" -ForegroundColor Gray
    } catch {
        Write-Host "      [错误] 替换失败: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host '[2/4] 启动服务...' -ForegroundColor Green
Start-Service -Name $service
(Get-Service $service).WaitForStatus('Running', '00:01:00')
Write-Host "      $((Get-Service $service).Status)" -ForegroundColor Gray

Write-Host '[3/4] 等待就绪...' -ForegroundColor Green
Start-Sleep -Seconds 2

# --- 端口探测 ---
$port = 3001
$cfg = Join-Path $baseDir 'config.json'
if (Test-Path $cfg) {
    try { $port = (Get-Content $cfg -Raw | ConvertFrom-Json).port } catch { }
}
try {
    $resp = Invoke-WebRequest -Uri "http://127.0.0.1:$port/" -UseBasicParsing -TimeoutSec 5
    Write-Host "      页面: HTTP $($resp.StatusCode)  (端口 $port)" -ForegroundColor Gray
} catch {
    Write-Host "      页面探测失败: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host '[4/4] 自检：调用定位接口...' -ForegroundColor Green
# 取一篇真实文章路径做端到端验证。
# 注意：不能把 JSON 当字符串交给 Invoke-WebRequest —— 它按 ISO-8859-1 编码请求体，
# 中文路径会被破坏，服务端只会返回"文件不存在"，从而误判为功能故障。
$testPath = $null
try {
    $wc = New-Object System.Net.WebClient
    $wc.Encoding = [System.Text.Encoding]::UTF8
    $tree = $wc.DownloadString("http://127.0.0.1:$port/api/tree") | ConvertFrom-Json

    $stack = New-Object System.Collections.Stack
    $tree.children | ForEach-Object { $stack.Push($_) }
    while ($stack.Count -gt 0 -and -not $testPath) {
        $node = $stack.Pop()
        if ($node.type -eq 'file') { $testPath = $node.path }
        elseif ($node.children) { $node.children | ForEach-Object { $stack.Push($_) } }
    }
} catch { }

if ($testPath) {
    # 显式转 UTF-8 字节，保证中文路径原样送达服务端
    $json  = '{"path":' + (ConvertTo-Json $testPath -Compress) + '}'
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)

    $req = [System.Net.HttpWebRequest]::Create("http://127.0.0.1:$port/api/reveal")
    $req.Method        = 'POST'
    $req.ContentType   = 'application/json'
    $req.ContentLength = $bytes.Length
    $stream = $req.GetRequestStream()
    $stream.Write($bytes, 0, $bytes.Length)
    $stream.Close()

    try {
        $resp   = $req.GetResponse()
        $reader = New-Object System.IO.StreamReader($resp.GetResponseStream(), [System.Text.Encoding]::UTF8)
        $content = $reader.ReadToEnd()
        $reader.Close(); $resp.Close()
        Write-Host "      定位成功（HTTP $([int]$resp.StatusCode)）" -ForegroundColor Green
        Write-Host "      资源管理器应已在桌面弹出并选中该文件。" -ForegroundColor Green
    } catch [System.Net.WebException] {
        $er = $_.Exception.Response
        if ($er) {
            $reader = New-Object System.IO.StreamReader($er.GetResponseStream(), [System.Text.Encoding]::UTF8)
            $content = $reader.ReadToEnd()
            $reader.Close()
            Write-Host "      定位失败（HTTP $([int]$er.StatusCode)）" -ForegroundColor Yellow
            Write-Host "      响应: $content" -ForegroundColor Yellow
        } else {
            Write-Host "      定位失败: $($_.Exception.Message)" -ForegroundColor Yellow
        }
        Write-Host "      详细日志: $(Join-Path $logDir 'stdout.log')" -ForegroundColor Gray
    }
} else {
    Write-Host '      跳过（未取到测试文章）' -ForegroundColor Gray
}

Write-Host ''
Write-Host '============================================' -ForegroundColor Cyan
Write-Host '  完成。请强制刷新浏览器（Ctrl+F5）后点击定位按钮。' -ForegroundColor Cyan
Write-Host '============================================' -ForegroundColor Cyan
Write-Host ''
Read-Host '按回车退出'
