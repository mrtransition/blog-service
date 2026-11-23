# Python脱离CMD启动

关闭 CMD 窗口后程序退出的原因是：**Python 脚本作为控制台进程的子进程，当父进程（CMD）终止时，子进程也会被系统关闭**。  

要让程序脱离 CMD 独立运行，可以使用 **`pythonw.exe`**（无控制台窗口的 Python 解释器）来启动脚本。

---

### ✅ 解决方案（任选一种）

#### 1. 直接使用 `pythonw.exe` 运行（推荐）
打开 **“运行”**（`Win + R`）或**文件资源管理器地址栏**，输入：
```cmd
pythonw DesktopTodoApp.py
```
或完整路径：
```cmd
pythonw C:\你的路径\DesktopTodoApp.py
```
这样程序将在后台启动，没有黑窗口，关闭 CMD 也不会影响它。

---

#### 2. 创建一个启动批处理（双击运行）
新建一个文本文件，写入以下内容，保存为 `启动待办.bat`（扩展名 `.bat`）：
```batch
@echo off
start "" pythonw "D:\ProgramFiles\todo\DesktopTodoApp.py"
```
**双击这个 bat 文件**，程序即会在后台运行，且不依赖任何命令行窗口。

> 如果你的 Python 路径不在系统 PATH 中，需要写完整路径，例如：  
> `start "" "C:\Users\你的用户名\AppData\Local\Programs\Python\Python311\pythonw.exe" "D:\ProgramFiles\todo\DesktopTodoApp.py"`

---

#### 3. 创建 VBS 脚本（完全无窗口启动）
如果 `pythonw` 仍然闪现窗口，可以用 VBS 启动：
```vbscript
CreateObject("WScript.Shell").Run "pythonw D:\ProgramFiles\todo\DesktopTodoApp.py", 0, False
```
保存为 `.vbs` 文件，双击运行。

---

### 🖥️ 如何让程序开机自动启动并驻留
1. 将上述 **bat 或快捷方式** 放入 Windows 启动文件夹：  
   按 `Win + R`，输入 `shell:startup`，回车。
2. 把 `启动待办.bat`（或指向 `pythonw` 的快捷方式）复制到该文件夹。

---

### 🧷 确保程序关闭窗口后隐藏到托盘
你的代码中已实现该功能，但需要保证安装了 `pystray` 和 `Pillow`：
```bash
pip install pystray Pillow
```
这样点击窗口的 `✕` 按钮时，程序会**隐藏到系统托盘**而不是退出。只有右键托盘图标选择“退出”才会彻底关闭。

---

### 📌 总结
- **一次运行**：双击 `启动待办.bat` 或在命令行用 `pythonw`。
- **开机自启**：把快捷方式放入 `shell:startup`。
- **不依赖 CMD**：`pythonw` 运行后，程序独立存在，关闭任何命令行窗口都不会影响它。

如果仍有问题，请告诉我你的 Python 安装路径，我可以帮你写出准确的启动命令。