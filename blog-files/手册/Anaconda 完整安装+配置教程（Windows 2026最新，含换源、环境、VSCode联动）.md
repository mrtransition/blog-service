# Anaconda 完整安装+配置教程（Windows 2026最新，含换源、环境、VSCode联动）
## 一、Anaconda 是什么
Anaconda 是Python科学计算环境管理器，自带conda包管理、虚拟环境、数据分析全套库（numpy/pandas/matplotlib），区分两个版本：
1. **Anaconda3（完整版）**：900M，预装几百个数据分析包，新手推荐
2. **Miniconda（精简版）**：50M，仅Python+conda，按需装包，省空间

## 二、下载安装（Windows）
### 1. 下载地址
官网：https://www.anaconda.com/products/distribution#download-section
国内镜像高速下载：清华镜像 https://mirrors.tuna.tsinghua.edu.cn/anaconda/archive/
选择：Windows 64-bit Graphical Installer

### 2. 安装关键步骤（避坑重点）

1. Next → I Agree
2. 安装类型：**Just Me (推荐)**，无需管理员权限
3. 安装路径（重中之重）
   - 禁止C盘、禁止中文、禁止空格路径
   - 推荐：`D:\anaconda3`，磁盘预留≥6GB空间
4. 高级选项勾选（两个全选）
   - Create shortcuts：创建开始菜单快捷方式
   - Register Anaconda3 as system Python：系统识别Anaconda Python（VSCode自动识别环境必备）
5. Install 等待5~15分钟，完成后一路Next，取消勾选启动Navigator直接Finish

### 3. 验证安装成功
打开开始菜单 → **Anaconda Prompt**
输入：
```bash
conda -V
python -V
conda list
```
输出版本号即安装正常。

## 三、核心配置1：系统环境变量（cmd/powershell直接用conda）
安装时未勾选添加PATH，手动配置3个路径（替换为你的安装目录）：
```
D:\anaconda3
D:\anaconda3\Scripts
D:\anaconda3\Library\bin
```
操作：此电脑→属性→高级系统设置→环境变量→系统变量Path→新建添加3条，重启终端生效。

## 四、核心配置2：国内镜像源（解决下载慢/失败，清华源）
打开Anaconda Prompt，一次性执行全部命令：
```bash
# 清空原有国外源
conda config --remove-key channels
# 添加清华全通道
conda config --add channels https://mirrors.tuna.tsinghua.edu.cn/anaconda/pkgs/main
conda config --add channels https://mirrors.tuna.tsinghua.edu.cn/anaconda/pkgs/free
conda config --add channels https://mirrors.tuna.tsinghua.edu.cn/anaconda/cloud/conda-forge
conda config --add channels https://mirrors.tuna.tsinghua.edu.cn/anaconda/cloud/pytorch
conda config --add channels https://mirrors.tuna.tsinghua.edu.cn/anaconda/cloud/bioconda
# 显示下载来源
conda config --set show_channel_urls yes
```
### 配套pip换源（pip下载加速）
```bash
pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple
```
### 验证源配置
```bash
conda config --show
```
出现清华镜像地址即成功。

## 五、核心配置3：虚拟环境管理（多Python版本隔离）
### 1. 查看所有环境
```bash
conda env list
# 等价
conda info --envs
```
### 2. 创建新环境（指定Python版本）
```bash
# 创建名为py310，python3.10环境
conda create -n py310 python=3.10
# 创建同时预装数据分析库
conda create -n data python=3.10 numpy pandas matplotlib
```
### 3. 激活/退出环境
```bash
conda activate py310  # 进入环境，终端前缀显示(py310)
conda deactivate       # 退出回到base基础环境
```
### 4. 删除无用环境（必须先退出）
```bash
conda remove -n py310 --all
# 简写
conda env remove -n py310
```
### 5. 环境克隆
```bash
conda create -n newenv --clone py310
```

## 六、包管理常用命令
```bash
conda list                  # 查看当前环境所有包
conda install numpy=1.26    # 安装指定版本包
conda remove numpy          # 卸载包
conda update conda          # 更新conda本体
conda update --all          # 更新当前环境全部包
conda clean --all           # 清理缓存，释放十几GB空间
conda search torch          # 查询可安装版本
```

## 七、VSCode 联动Anaconda（开发必备）
1. VSCode安装插件：**Python**（微软官方）
2. 打开项目文件夹，右下角点击Python版本选择器
3. 列表自动识别所有conda虚拟环境，选中即可切换解释器
4. 终端自动加载对应conda环境，`pip/conda`命令直接使用

## 八、常见踩坑解决
1. **cmd输入conda提示不是内部命令**
   → 未配置3条PATH环境变量，添加后重启终端
2. **下载包速度几KB/s**
   → 执行上面清华换源全套命令
3. **创建环境报错内存不足**
   → 执行`conda clean --all`清理缓存
4. **VSCode找不到conda环境**
   → 安装时勾选`Register Anaconda as system Python`，重启VSCode
5. **中文路径报错/乱码**
   → 重装，路径全程无中文、空格

## 九、极简新手一套流程
1. D盘安装Anaconda3，勾选注册系统Python
2. Prompt执行清华conda+pip换源
3. 创建专用环境：`conda create -n dev python=3.10`
4. VSCode选择dev环境，开始开发