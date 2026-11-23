# Hermes Agent + Ollama 本地部署专业配置与使用手册
**适用环境**：Windows 10 / Windows 11 原生 PowerShell 环境  
**核心能力**：纯离线本地 AI 智能体，支持文件读写、终端命令执行、代码运行、批量文本处理，全程无云端 API 调用，数据本地闭环。

---

## 第一章 方案概述
### 1.1 核心优势
1. **完全离线**：模型推理、工具执行全部本地完成，无数据外泄风险
2. **零成本**：无 API 调用费用，依赖开源模型与开源框架
3. **强工具能力**：原生支持 Function Call，可自然语言驱动系统操作
4. **高度可定制**：支持自定义模型、工作目录、工具权限、运行环境

### 1.2 硬件要求参考
| 模型规格 | 最低显存 | 推荐显存 | 最低内存 |
|--------|--------|--------|--------|
| 7B / 8B 量化版 | 4GB | 6GB | 8GB |
| 7B / 8B 完整版 | 6GB | 8GB | 16GB |
| 32B 量化版 | 12GB | 16GB | 32GB |
| 32B 完整版 | 20GB | 24GB | 64GB |

### 1.3 推荐适配模型（原生支持工具调用）
- 轻量首选：`qwen2.5:7b-q4_K_M`、`hermes3:8b`
- 均衡首选：`qwen3.5:32b-q4_K_M`
- 专业工具向：`hermes3:70b-q4_K_M`

---

## 第二章 前置环境部署
### 2.1 Git for Windows 安装（必做）
Hermes 安装脚本依赖 Git 拉取源码，缺失会直接报 `git 无法识别` 错误。
1. 官网下载：https://git-scm.com/download/win
2. 安装关键配置：
   - 选择 **Git from the command line and also from 3rd-party software**（自动写入系统 PATH）
   - 其余选项保持默认即可
3. 验证：关闭所有终端，新开管理员 PowerShell 执行
```powershell
git --version
```
输出版本号即为安装成功。

### 2.2 Ollama 本地模型服务部署
1. 官网下载 Windows 安装包：https://ollama.com/download/windows
2. 安装完成后默认开机自启，后台常驻 11434 端口，**无需手动执行 `ollama serve`**
3. 拉取目标模型（示例）
```powershell
# 轻量中文工具模型推荐
ollama pull qwen2.5:7b-q4_K_M
# 原生Hermes工具模型
ollama pull hermes3:8b
```
4. 服务连通性验证
```powershell
curl http://127.0.0.1:11434/v1/models
```
返回模型列表 JSON 即服务正常。

---

## 第三章 Hermes Agent 安装部署
### 3.1 旧环境清理（安装失败必做）
管理员 PowerShell 执行，彻底清除残留文件与环境变量：
```powershell
# 删除默认C盘程序目录
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\hermes" -ErrorAction SilentlyContinue
# 删除默认配置目录
Remove-Item -Recurse -Force "$env:USERPROFILE\.hermes" -ErrorAction SilentlyContinue
# 清理PATH环境变量残留
[Environment]::SetEnvironmentVariable("PATH", ($env:PATH -split ';' | Where-Object { $_ -notmatch 'hermes' }) -join ';', "User")
```

### 3.2 自定义目录安装（推荐，不占用C盘）
以下示例安装至 `D:\AI\Hermes`，可自行替换路径：
```powershell
# 1. 设置配置数据永久存储路径
[Environment]::SetEnvironmentVariable("HERMES_HOME_DATA", "D:\AI\Hermes\.hermes-data", "User")
$env:HERMES_HOME_DATA = "D:\AI\Hermes\.hermes-data"

# 2. 执行自定义目录安装
iex (irm https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.ps1) -ArgumentList `
    -InstallDir "D:\AI\Hermes\agent" `
    -HermesHome "D:\AI\Hermes\home"
```
参数说明：
- `-InstallDir`：核心源码、Python 虚拟环境、依赖文件存放目录
- `-HermesHome`：可执行文件、内置工具、启动脚本存放目录
- `HERMES_HOME_DATA`：配置文件、会话记录、缓存数据存放目录

### 3.3 安装验证
1. 关闭所有 PowerShell 窗口，重新打开管理员终端
2. 执行命令验证
```powershell
# 验证命令路径
Get-Command hermes
# 环境全量自检
hermes doctor
```
无红色报错即为安装成功。

---

## 第四章 Ollama 对接核心配置
### 4.1 交互式配置全流程（新手推荐）
执行命令进入配置向导：
```powershell
hermes model
```

| 步骤 | 选项 | 配置值 | 说明 |
|------|------|--------|------|
| 1 选择服务商 | 38. Custom endpoint (enter URL manually) | - | 对接本地自建接口，不可选26.Ollama Cloud（云端服务） |
| 2 接口地址 | Base URL | `http://127.0.0.1:11434/v1` | Ollama 本地服务标准 OpenAI 兼容接口 |
| 3 API密钥 | API Key | `ollama` | 本地服务无鉴权，任意非空字符串均可 |
| 4 模型名称 | Model Name | 本地已拉取模型名，如 `qwen2.5:7b-q4_K_M` | 必须与 `ollama list` 中名称完全一致 |
| 5 上下文长度 | Context length | `65536` | 工具调用最低要求，自动检测常识别失败导致函数调用失效 |
| 6 兼容模式 | API compatibility mode | 2. Chat Completions | Ollama 完全兼容 OpenAI 对话接口标准，手动指定比自动检测更稳定 |
| 7 显示名称 | Display name | 自定义，如 `Ollama-qwen2.5-7b` | 多模型切换时便于区分 |

### 4.2 配置文件直写配置（进阶）
直接编辑配置文件，适合批量部署与精准调优：
1. 打开配置文件
```powershell
hermes config edit
```
2. 完整可用配置模板（带注释）
```yaml
llm:
  provider: custom
  base_url: http://127.0.0.1:11434/v1
  api_key: ollama
  model: qwen2.5:7b-q4_K_M
  context_length: 65536
  compatibility_mode: chat_completions
  temperature: 0.2      # Agent场景调低随机性，提升工具调用稳定性
  max_tokens: 4096      # 单次回复最大token数
  timeout: 300          # 推理超时时间，本地模型建议设为300秒
  ollama_compatible: true  # 开启Ollama专属兼容优化

system:
  auto_tools: true      # 自动启用文件、终端、代码等内置工具
  safe_mode: false      # 关闭安全模式，允许执行系统命令
  verbose_tools: false  # 关闭工具详细日志，保持界面简洁

terminal_backend: local  # Windows本地直接执行命令，无需容器
gateway_workdir: .       # 网关工作根目录，.为当前终端目录
sudo_support: false      # Windows无sudo，直接关闭
```

---

## 第五章 标准化使用指南
### 5.1 基础启动与退出
```powershell
# 标准交互模式
hermes

# TUI分栏模式（左侧对话+右侧工具日志）
hermes --tui

# 单次提问，不进入持续会话
hermes chat -q "读取当前目录所有txt并总结"
```
对话内输入 `/exit` 或按 `Ctrl+C` 退出。

### 5.2 全局管理命令汇总
| 命令 | 功能 |
|------|------|
| `hermes model` | 模型配置与切换 |
| `hermes doctor` | 环境健康自检 |
| `hermes config` | 查看当前完整配置 |
| `hermes config edit` | 编辑配置文件 |
| `hermes config path` | 查看配置文件路径 |
| `hermes sessions list` | 查看历史会话列表 |
| `hermes --resume 会话ID` | 恢复指定历史会话 |
| `hermes update` | 更新Hermes本体 |
| `hermes gateway run` | 启动网页网关服务 |
| `hermes gateway stop` | 停止网关服务 |

### 5.3 对话内快捷指令
进入 `❯` 对话界面后可用，输入 `/` 可唤起补全：
| 指令 | 功能 |
|------|------|
| `/help` | 查看全部内置指令 |
| `/tools` | 查看当前已启用的工具列表 |
| `/model 模型名` | 临时切换模型（当前会话生效） |
| `/model 模型名 --global` | 切换并设为全局默认模型 |
| `/clear` | 清空当前会话上下文 |
| `/rollback` | 回滚上一轮工具操作 |
| `/verbose` | 开关工具调用详细日志 |
| `/bg 指令` | 后台执行任务，不阻塞对话 |
| `/queue 内容` | 排队发送消息，不打断当前推理 |
| `/exit` | 退出对话 |

### 5.4 核心工具场景示例
Hermes 支持自然语言直接调用工具，无需手动输入命令。

#### 1. 文件与文本处理
```
读取D:\笔记\项目方案.txt，提炼核心要点输出为markdown
遍历当前目录所有log文件，筛选包含error的行并保存到error_log.txt
批量重命名桌面所有txt文件，统一添加前缀202606_
搜索D盘所有md文件，查找包含"Ollama"的文件路径
```

#### 2. Windows 系统操作
```
查看C盘剩余空间，列出占用最大的5个文件夹
查询当前所有运行进程，筛选ollama相关进程
创建D:\AI\Workspace目录，并在其中新建3个空白txt文件
清空系统临时文件夹
```

#### 3. 代码执行与数据处理
```
写Python脚本，批量统计当前目录所有txt的字数并生成统计表
解析当前目录的data.csv，输出各列数据的统计结果
用正则批量替换所有txt中的指定字符串
```

---

## 第六章 性能与稳定性优化
### 6.1 Ollama 侧优化
1. **优先用量化模型**：同参数量化版速度提升 2~3 倍，显存占用减半，工具调用精度损失极小
2. **设置显存占用上限**：系统环境变量新增 `OLLAMA_NUM_GPU` 与 `OLLAMA_MAX_LOADED_MODELS`，控制多模型显存占用
3. **大内存设备开启内存映射**：默认已开启，16GB 以上内存可显著提升加载速度

### 6.2 Hermes 侧优化
1. **合理设置上下文长度**：日常工具任务 65536 足够，超长文档处理可上调至 131072，避免无意义的显存浪费
2. **Agent 场景调低温度**：`temperature: 0.1~0.3`，降低输出随机性，提升工具调用准确率
3. **延长超时时间**：本地推理速度慢于云端，`timeout` 建议设为 300s，避免推理中途断开

### 6.3 系统侧优化
1. 以管理员身份运行 PowerShell，避免文件读写权限不足
2. 关闭 Windows Defender 实时保护对 Ollama 进程的扫描，提升推理速度
3. 模型文件建议放在 SSD 上，加载速度显著提升

---

## 第七章 常见故障排查手册
### 7.1 安装类问题
**报错：无法将“git”项识别为 cmdlet**
- 原因：未安装 Git 或未加入系统 PATH
- 解决：安装 Git for Windows 并勾选添加到 PATH，重启终端后重装

**报错：hermes 命令找不到**
- 原因：环境变量未刷新
- 解决：关闭所有 PowerShell 窗口重新打开；若仍无效，手动将 `HermesHome\bin` 加入系统 PATH

### 7.2 连接类问题
**报错：listen tcp 0.0.0.0:11434: bind 地址已占用**
- 原因：Ollama 已后台自启，重复执行 `ollama serve`
- 解决：无需手动启动服务，直接验证接口连通性即可；需重启则先杀掉占用进程再启动

**报错：连接Ollama超时 / 模型无响应**
1. 确认 Ollama 托盘图标存在，服务正常运行
2. 执行 `curl http://127.0.0.1:11434/v1/models` 验证接口
3. 检查防火墙是否放行 11434 端口
4. 配置文件中统一使用 `127.0.0.1`，不使用 `localhost`

### 7.3 功能类问题
**现象：AI 只纯文字回答，不调用工具**
1. 确认 `context_length ≥ 65536`，短上下文无法承载函数调用协议
2. 确认 `auto_tools: true`，执行 `/tools` 查看工具是否启用
3. 更换支持原生工具调用的模型，部分基础模型无 Function Call 能力

**现象：终端命令输出中文乱码**
- 解决：PowerShell 执行以下命令后再启动 Hermes
```powershell
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
```

**现象：文件读写提示权限不足**
- 解决：以管理员身份运行 PowerShell，再执行 `hermes` 命令

### 7.4 运行类问题
**现象：推理进度条0%卡住，长时间无输出**
1. 按 `Ctrl+C` 中断当前请求
2. 检查显存/内存占用，确认硬件足够运行当前模型
3. 更换量化版模型，降低硬件负载
4. 增大配置文件中 `timeout` 参数值

**现象：新消息打断当前推理，出现 interrupt 提示**
- 正常机制：模型推理中发送新消息会自动中断上一轮
- 规避方法：使用 `/queue 消息内容` 排队发送，或等待推理完成再发新消息

---

## 第八章 卸载与环境重置
### 8.1 保留配置卸载（仅删程序）
```powershell
hermes uninstall
```
选择 `Keep Data`，后续重装无需重新配置 Ollama 接口。

### 8.2 彻底全量卸载
管理员 PowerShell 一键执行（替换为你的自定义安装路径）：
```powershell
# 停止进程与服务
hermes gateway stop
taskkill /f /im hermes.exe -ErrorAction SilentlyContinue

# 删除程序与配置文件
Remove-Item -Recurse -Force "D:\AI\Hermes" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "$env:USERPROFILE\.hermes" -ErrorAction SilentlyContinue

# 清理环境变量
[Environment]::SetEnvironmentVariable("HERMES_HOME_DATA", $null, "User")
[Environment]::SetEnvironmentVariable("PATH", ($env:PATH -split ';' | Where-Object { $_ -notmatch 'hermes' }) -join ';', "User")

# 清理计划任务
Get-ScheduledTask | Where-Object {$_.TaskName -match "hermes"} | Unregister-ScheduledTask -Confirm:$false
```

### 8.3 配置重置（不卸载程序）
删除配置目录后重启 Hermes，自动恢复初始状态：
```powershell
Remove-Item -Recurse -Force "D:\AI\Hermes\.hermes-data"
```

---

## 附录：快速上手速查表
1. 打开管理员 PowerShell
2. 确认 Ollama 后台运行
3. 输入 `hermes` 进入对话
4. 直接下达文件/系统操作指令
5. 切换模型执行 `hermes model`
6. 排查问题执行 `hermes doctor`
7. 退出输入 `/exit`