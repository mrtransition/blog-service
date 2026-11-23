# Windows & Linux 端口占用完整排查+解决方案
## 一、Linux 系统（CentOS / Ubuntu / Debian）
### 1. 查找占用端口进程
#### 方式1：lsof（推荐，直观）
```bash
# 查看8080端口占用
lsof -i :8080
# 只输出PID
lsof -t -i :8080
```
字段说明：`PID` 为进程ID，`COMMAND` 程序名

#### 方式2：netstat（通用）
```bash
netstat -tulpn | grep 8080
```
`tcp6` 是IPv6端口，最后一列 `PID/程序名`

#### 方式3：ss（高效，替代netstat）
```bash
ss -tulnp | grep 8080
```

### 2. 杀掉占用端口进程
```bash
# 普通终止
kill -15 进程PID
# 强制杀死（卡死无响应时用）
kill -9 进程PID

# 一键杀端口（组合命令）
kill -9 $(lsof -t -i:8080)
```

### 3. 常见端口占用根源&规避
1. 程序未正常关闭，后台残留进程
2. Tomcat/Nginx/MySQL重复启动
3. 服务配置固定端口，多实例冲突
4. 防火墙放行但端口被其他程序抢占

### 4. 临时端口冲突替代方案
修改项目/服务配置端口号，重启服务；
如需永久释放：停止对应系统服务
```bash
# 示例：停止nginx
systemctl stop nginx
```

---

## 二、Windows 系统（Win10/Win11/Server）
### 1. CMD 查找端口占用
#### 步骤1：查询端口对应PID
```cmd
netstat -ano | findstr "8080"
```
输出最后一列数字 = PID

#### 步骤2：通过PID查是什么程序
```cmd
tasklist | findstr "1234"  # 1234替换为上面查到的PID
```

### 2. PowerShell 更简洁命令
```powershell
# 直接查看端口+进程名
netstat -ano | Where-Object {$_ -match ':8080'}
# 根据PID杀进程
taskkill /F /PID 1234
# 一键释放端口（PowerShell）
netstat -ano | findstr ":8080" | ForEach-Object { $a=$_.Split()[-1]; taskkill /F /PID $a }
```

### 3. 图形化快速定位（任务管理器）
1. Ctrl+Shift+Esc 打开任务管理器
2. 详细信息 → 右键列 → 勾选 **PID（进程标识符）**
3. 根据查到的PID找到程序，右键结束任务

### 4. 强制关闭进程命令
```cmd
# /F 强制终止，/T 连带子进程一起杀
taskkill /F /T /PID 1234
```

### 5. Windows WSL 端口占用特殊情况
WSL占用Windows端口，关闭WSL发行版：
```powershell
wsl --shutdown
```

---

## 三、通用最优解决方案（分场景）
### 场景1：开发调试（Java/Node/Python本地服务）
1. 先正常停止程序（IDE停止按钮、Ctrl+C）
2. 仍占用 → 执行对应系统杀端口命令
3. 根治：代码加关闭钩子，程序退出自动释放端口

### 场景2：服务器正式服务（Nginx、数据库、中间件）
**禁止直接kill -9 / taskkill强杀！**
优先使用服务管理命令优雅停止：
```bash
# Linux
systemctl stop mysql
systemctl stop tomcat

# Windows服务
net stop mysql80
```
优雅停止失败再强制杀进程。

### 场景3：端口被系统内置程序占用（80、443、3306）
- Windows：IIS、SQL Server、迅雷、QQ、虚拟机占用80/443
- Linux：Apache、httpd、nginx占用80
解决：停止对应系统服务，或更换业务端口

### 场景4：端口占用反复出现
1. 开机自启程序占用：取消开机启动
2. 定时脚本重复启动：检查crontab(Linux)/任务计划(Windows)
3. 容器Docker占用端口
```bash
# Linux查看容器端口映射
docker ps | grep 8080
# 停止容器
docker stop 容器ID
```

---

## 四、排错速查表
| 系统 | 查询端口 | 一键释放端口 |
|------|--------|------------|
| Linux | lsof -i:端口 | kill -9 $(lsof -t -i:端口) |
| Windows CMD | netstat -ano | taskkill /F /T /PID PID值 |
| Windows PowerShell | netstat -ano | netstat -ano|findstr ":端口"|%{$p=$_.split()[-1];taskkill /F /PID $p} |

## 补充：权限问题
- Linux：低于1024的特权端口（80/443）需要root/sudo执行查询、启停
- Windows：查询、杀进程提示拒绝访问 → 以**管理员身份**打开CMD/PowerShell