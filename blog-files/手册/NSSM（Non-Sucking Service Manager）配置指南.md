# NSSM（Non-Sucking Service Manager）配置指南

## 目录

1. [NSSM 简介](#nssm-简介)
2. [安装 NSSM](#安装-nssm)
3. [快速上手：将程序注册为服务](#快速上手将程序注册为服务)
4. [配置参数详解](#配置参数详解)
   - [Application（应用程序）](#application应用程序)
   - [Details（详细信息）](#details详细信息)
   - [Log on（登录账户）](#log-on登录账户)
   - [Dependencies（依赖关系）](#dependencies依赖关系)
   - [Process（进程管理）](#process进程管理)
   - [Shutdown（关闭策略）](#shutdown关闭策略)
   - [Exit actions（退出操作）](#exit-actions退出操作)
   - [I/O（输入输出重定向）](#io输入输出重定向)
   - [File rotation（日志轮转）](#file-rotation日志轮转)
   - [Environment（环境变量）](#environment环境变量)
5. [高级场景配置示例](#高级场景配置示例)
   - [Java / Python 脚本服务](#java--python-脚本服务)
   - [Node.js 应用服务](#nodejs-应用服务)
   - [带参数的后台进程](#带参数的后台进程)
   - [控制台程序静默运行](#控制台程序静默运行)
   - [优雅关闭与超时控制](#优雅关闭与超时控制)
   - [依赖服务与启动顺序](#依赖服务与启动顺序)
6. [服务管理命令参考](#服务管理命令参考)
7. [故障排除与日志诊断](#故障排除与日志诊断)
8. [安全与最佳实践](#安全与最佳实践)
9. [附录：NSSM 命令行常用示例](#附录nssm-命令行常用示例)

---

## NSSM 简介

NSSM（Non-Sucking Service Manager）是一个轻量级的服务封装工具，可将任何可执行程序（包括批处理脚本、解释型语言程序）注册为标准的 Windows 服务。相比于 Windows 内置的 `sc` 命令或 `srvany`，NSSM 提供了更精细的进程监控、自动重启、日志重定向、环境变量注入、优雅关闭等能力，且完全通过命令行或 GUI 配置，无需编写代码。

**核心特性：**

- 将任意 `.exe`、`.bat`、`.cmd`、`.ps1`、`.py`、`.jar` 等注册为 Windows 服务。
- 自动守护进程：程序崩溃后自动重启，支持延迟重启和退出动作定制。
- I/O 重定向：可将 stdout/stderr 写入文件，并支持日志轮转。
- 环境隔离：可为服务设置独立的环境变量。
- 兼容 Windows 服务控制管理器（SCM），支持依赖、恢复操作、运行账户指定。
- 提供 GUI 配置器（`nssm.exe`）和纯命令行两种管理方式。

---

## 安装 NSSM

1. 从 [NSSM 官方网站](https://nssm.cc/download) 下载最新稳定版压缩包。
2. 解压到固定目录，例如 `C:\nssm`（目录中包含 `nssm.exe`，分为 32 位和 64 位，选择与系统匹配的版本）。
3. （推荐）将 `nssm.exe` 所在目录添加到系统 `PATH` 环境变量，方便全局调用。

```powershell
# 查看版本确认可用
nssm version
```

---

## 快速上手：将程序注册为服务

以注册一个简单的 `C:\myapp\app.exe` 为例。

### 使用 GUI 配置（简单直观）
```powershell
nssm install MyService
```
在弹出的窗口中填写：
- **Application Path**：`C:\myapp\app.exe`
- **Startup directory**：`C:\myapp`
- **Arguments**：如有参数可填写（例如 `--port 8080`）

点击 "Install service" 即可完成注册。

### 使用命令行（自动化部署友好）
```powershell
nssm install MyService "C:\myapp\app.exe"
nssm set MyService AppDirectory "C:\myapp"
nssm set MyService AppParameters "--port 8080"
```

安装后启动服务：
```powershell
nssm start MyService
```

---

## 配置参数详解

NSSM 所有参数均可用 `nssm set <服务名> <参数> <值>` 设置，也可通过 GUI 修改。以下按配置面板分类说明。

### Application（应用程序）

| 参数 | 说明 |
|------|------|
| `Application` | 可执行文件完整路径。支持 `.exe`, `.bat`, `.cmd`, `.ps1`, `.py` 等（需关联解释器）。|
| `AppDirectory` | 工作目录，程序启动时的当前目录。建议设置为应用程序所在目录。 |
| `AppParameters` | 传递给应用程序的命令行参数。若路径含空格需用引号包裹整个参数字符串。 |
| `AppStdout` | stdout 重定向日志文件路径。未设置则丢弃输出。 |
| `AppStderr` | stderr 重定向日志文件路径。未设置则丢弃输出。 |
| `AppStdoutCreationDisposition` | 日志文件写入模式（覆盖/追加），默认覆盖。 |
| `AppStderrCreationDisposition` | 同上。 |
| `AppRotateFiles` | 启用日志轮转（见 I/O 节）。 |
| `AppRotateOnline` | 允许正在写入的日志被轮转。 |
| `AppRotateSeconds` | 轮转检查间隔（秒）。 |
| `AppRotateBytes` | 日志文件大小上限（字节），超过后轮转。 |
| `AppRotateHigh` | 保留的历史日志数量上限。 |

### Details（详细信息）

| 参数 | 说明 |
|------|------|
| `DisplayName` | 服务在 `services.msc` 中显示的名称。 |
| `Description` | 服务描述，显示在服务属性中。 |
| `Start` | 启动类型：`SERVICE_AUTO_START`（自动）、`SERVICE_DEMAND_START`（手动）、`SERVICE_DISABLED`（禁用）。 |
| `Type` | 服务类型，一般保持默认 `SERVICE_WIN32_OWN_PROCESS`。 |

### Log on（登录账户）

| 参数 | 说明 |
|------|------|
| `ObjectName` | 运行服务的账户，格式 `DOMAIN\user` 或 `.\LocalAccount`。默认为 `LocalSystem`。 |
| `ObjectPassword` | 账户密码（明文存储在注册表，需注意安全）。 |

**注意：** 本地服务账户（`NT AUTHORITY\LocalService`、`NT AUTHORITY\NetworkService`）仅需写 `ObjectName` 留空密码即可。

### Dependencies（依赖关系）

| 参数 | 说明 |
|------|------|
| `DependOnService` | 依赖的其他服务名称，多个用 `+` 分隔（如 `LanmanWorkstation+LanmanServer`）。 |
| `DependOnGroup` | 依赖的服务组，可为空。 |

### Process（进程管理）

| 参数 | 说明 |
|------|------|
| `AppPriority` | 进程优先级：`IDLE`, `BELOW_NORMAL`, `NORMAL`, `ABOVE_NORMAL`, `HIGH`, `REALTIME`。 |
| `AppNoWindow` | 1=禁止创建控制台窗口，适用于控制台程序静默运行。 |
| `AppAffinity` | CPU 关联性掩码，例如 `1` 表示仅用 CPU0。 |
| `AppThrottle` | 限制进程 CPU 使用率（需要进程本身支持）。通常留空。 |

### Shutdown（关闭策略）

| 参数 | 说明 |
|------|------|
| `AppStopMethodSkip` | 跳过发送关闭信号？默认 `0`。 |
| `AppStopMethodConsole` | 通过 `GenerateConsoleCtrlEvent` 发送 Ctrl+C（仅对控制台程序有效）。一般置 `1` 可尝试优雅关闭。 |
| `AppStopMethodWindow` | 发送 `WM_CLOSE` 消息到窗口。 |
| `AppStopMethodThreads` | 发送消息到所有线程。 |
| `AppStopTimeout` | 等待程序自行退出的超时时间（毫秒），超时后强制终止。默认 `1500`。 |

**建议组合：** 先尝试 Console Ctrl 事件，超时后终止。可设置：
```powershell
nssm set MyService AppStopMethodConsole 1
nssm set MyService AppStopTimeout 5000
```

### Exit actions（退出操作）

控制程序退出后的自动响应，实现故障自愈。

| 参数 | 说明 |
|------|------|
| `AppExit` | 默认退出操作：`Restart`（重启）、`Ignore`（忽略）、`Exit`（退出服务）。 |
| `AppExitDefault` | 如果没匹配到特定退出码，执行的默认操作。 |
| `AppExit_<exitcode>` | 针对特定退出码的操作。例如 `AppExit_0=Exit` 表示正常退出不重启；`AppExit_1=Restart` 表示错误码 1 时重启。 |

**常见配置示例：** 正常退出（退出码 0）停止服务，其他异常退出重启。
```powershell
nssm set MyService AppExitDefault Restart
nssm set MyService AppExit 0 Exit
```

### I/O（输入输出重定向）

重定向 stdin/stdout/stderr 到文件，便于捕获控制台输出日志。

| 参数 | 说明 |
|------|------|
| `AppStdin` | 指定 stdin 来源文件（通常留空）。 |
| `AppStdout` | 标准输出日志路径。 |
| `AppStderr` | 错误输出日志路径（可与 stdout 合并为同一文件）。 |
| `AppStdoutCreationDisposition` | 写入模式：`1`=追加，`2`=覆盖（默认）。 |
| `AppStderrCreationDisposition` | 同上。 |

**示例：** 将标准输出和错误都追加到同一个日志：
```powershell
nssm set MyService AppStdout "C:\logs\service.log"
nssm set MyService AppStderr "C:\logs\service.log"
nssm set MyService AppStdoutCreationDisposition 1
nssm set MyService AppStderrCreationDisposition 1
```

### File rotation（日志轮转）

启用日志轮转防止日志文件无限增大。

| 参数 | 说明 |
|------|------|
| `AppRotateFiles` | 设置为 `1` 开启轮转。 |
| `AppRotateOnline` | `1` 表示支持在线轮转（不中断服务重写）。 |
| `AppRotateSeconds` | 定时检查间隔（秒），默认 `86400`（一天）。 |
| `AppRotateBytes` | 超过指定字节数后立即轮转。例如 `10485760` (10 MB)。 |
| `AppRotateHigh` | 保留的历史日志数量，达到后删除最旧的。 |

**示例：** 日志大于 10 MB 即轮转，保留 5 个归档。
```powershell
nssm set MyService AppRotateFiles 1
nssm set MyService AppRotateBytes 10485760
nssm set MyService AppRotateHigh 5
```

### Environment（环境变量）

为服务进程设置额外的环境变量，替换或追加系统变量。

| 操作 | 命令 |
|------|------|
| 设置变量 | `nssm set MyService AppEnvironmentExtra "KEY=VALUE"` |
| 设置多个变量 | 在同一参数中每行一个，如 `"PATH=%PATH%;C:\MyLib"` |

**注意：** 环境变量在 GUI 中可多行编辑，命令行需要包含换行符（PowerShell 使用 `"line1`nline2"`）。

---

## 高级场景配置示例

### Java / Python 脚本服务

当程序不是原生可执行文件时，需要将解释器路径作为 Application，脚本和参数放在 AppParameters 中。

**Java 示例：**
```powershell
nssm install MyJavaApp "C:\Program Files\Java\jdk-17\bin\java.exe"
nssm set MyJavaApp AppDirectory "C:\myapp"
nssm set MyJavaApp AppParameters "-jar myapp.jar --server.port=8080"
nssm set MyJavaApp AppStdout "C:\logs\java-app.log"
```

**Python 示例：**
```powershell
nssm install MyPythonService "C:\Python39\python.exe"
nssm set MyPythonService AppDirectory "C:\scripts"
nssm set MyPythonService AppParameters "my_service.py --prod"
```

**PowerShell 脚本：**
```powershell
nssm install MyPSScript "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"
nssm set MyPSScript AppParameters "-ExecutionPolicy Bypass -File C:\scripts\monitor.ps1"
```

### Node.js 应用服务

```powershell
nssm install NodeApp "C:\Program Files\nodejs\node.exe"
nssm set NodeApp AppDirectory "C:\nodeapp"
nssm set NodeApp AppParameters "server.js"
nssm set NodeApp AppStdout "C:\logs\node.log"
nssm set NodeApp AppStderr "C:\logs\node-error.log"
```

### 带参数的后台进程

参数中包含引号时，需整体转义。建议使用 PowerShell 的单引号/双引号组合避免解析问题。

```powershell
nssm set MyService AppParameters '--config "C:\my app\config.json" --port 443'
```

### 控制台程序静默运行

避免弹窗，并禁用控制台窗口创建：

```powershell
nssm set MyService AppNoWindow 1
```

### 优雅关闭与超时控制

向控制台程序发送 Ctrl+C 信号，等待 10 秒后强杀：

```powershell
nssm set MyService AppStopMethodConsole 1
nssm set MyService AppStopTimeout 10000
```

对于 GUI 程序可尝试发送窗口消息：
```powershell
nssm set MyService AppStopMethodWindow 1
```

### 依赖服务与启动顺序

确保在网络和数据库服务启动后再启动自身服务：

```powershell
nssm set MyService DependOnService "MSSQLSERVER+LanmanServer"
```

---

## 服务管理命令参考

| 命令 | 说明 |
|------|------|
| `nssm install <服务名> <程序路径>` | 安装服务（不带参数会弹出 GUI） |
| `nssm remove <服务名> confirm` | 删除服务（需 `confirm` 确认） |
| `nssm start <服务名>` | 启动服务 |
| `nssm stop <服务名>` | 停止服务 |
| `nssm restart <服务名>` | 重启服务 |
| `nssm status <服务名>` | 查询服务状态 |
| `nssm set <服务名> <参数> <值>` | 设置参数 |
| `nssm get <服务名> <参数>` | 获取参数值 |
| `nssm dump <服务名>` | 导出服务配置（用于备份/迁移） |
| `nssm edit <服务名>` | 打开 GUI 编辑服务配置 |
| `nssm rotate <服务名>` | 强制立即进行日志轮转 |

---

## 故障排除与日志诊断

1. **服务无法启动，状态为“启动后停止”**  
   - 检查应用程序路径和工作目录是否正确。  
   - 查看 NSSM 自身日志或应用程序日志：在事件查看器（`eventvwr.msc`）的“Windows 日志 → 应用程序”中查找来源为 `NSSM` 的事件。  
   - 临时启用 `AppStdout`/`AppStderr` 捕获输出以诊断。

2. **程序崩溃但未自动重启**  
   - 检查退出操作配置：`AppExitDefault` 是否为 `Restart`。  
   - 确认 `AppExit_<code>` 没有覆盖为 `Exit` 或 `Ignore`。  
   - 查看 NSSM 事件日志中是否有退出码记录。

3. **服务未按期望顺序启动**  
   - 确认 `DependOnService` 中依赖的服务名称正确（区分大小写）。  
   - 检查依赖服务本身是否已设置为自动启动。

4. **日志文件不写入或不轮转**  
   - 权限：运行服务的账户是否拥有目标日志目录的写入权限。  
   - 文件路径不存在：NSSM 不会自动创建目录，需预先创建。  
   - 轮转配置：`AppRotateFiles` 必须为 `1`，并设置 `AppRotateBytes` 或 `AppRotateSeconds`。

5. **查看已安装服务的完整配置**  
   ```powershell
   nssm dump MyService
   ```

---

## 安全与最佳实践

- **最小权限原则**：避免使用 `LocalSystem`，为服务创建专用本地账户或 gMSA，仅授予必要权限。
- **密码管理**：若使用专用账户，注意密码以可逆加密存储在注册表（`HKLM\SYSTEM\CurrentControlSet\Services\<服务名>\Parameters`）。定期更换密码，并使用受控权限限制对注册表项的访问。
- **日志防护**：确保日志目录仅允许服务账户和管理员写入，防止敏感信息泄露。
- **避免 PATH 注入**：使用 `AppEnvironmentExtra` 追加环境变量时，仔细校验路径值。
- **配置备份与迁移**：使用 `nssm dump <服务名> > backup.bat` 导出为批处理，可在新机器上通过执行该批处理恢复服务配置。
- **更新 NSSM**：新版 NSSM 修复了旧版本的一些本地提权漏洞，请及时更新并校验下载哈希值。

---

## 附录：NSSM 命令行常用示例

```powershell
# 安装服务
nssm install MyService "C:\myapp\app.exe"
nssm set MyService AppDirectory "C:\myapp"
nssm set MyService AppParameters "--config config.yml"
nssm set MyService DisplayName "My Application Service"
nssm set MyService Description "Handles background processing"
nssm set MyService Start SERVICE_AUTO_START

# 进程监控与退出动作
nssm set MyService AppExitDefault Restart
nssm set MyService AppExit 0 Exit
nssm set MyService AppExit 1 Restart

# 日志与轮转
nssm set MyService AppStdout "C:\logs\myservice.log"
nssm set MyService AppStderr "C:\logs\myservice_error.log"
nssm set MyService AppRotateFiles 1
nssm set MyService AppRotateBytes 5242880
nssm set MyService AppRotateHigh 3

# 环境变量
nssm set MyService AppEnvironmentExtra "JAVA_HOME=C:\java\jdk17`nMY_VAR=production"

# 启动与状态检查
nssm start MyService
nssm status MyService
```

> `nssm dump` 可以快速生成所有 `set` 命令，适合创建部署脚本。

---

通过以上配置，您可以可靠地将几乎所有 Windows 应用程序封装为系统服务，享受自动恢复、日志管理和标准服务生命周期控制带来的便利。如有更多细节需求，请参考 [NSSM 官方文档](https://nssm.cc/usage)。