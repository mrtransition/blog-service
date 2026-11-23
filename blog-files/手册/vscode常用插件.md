# VSCode 插件全分类推荐（2026最新，含你关注的Markdown、界面美化、开发效率）
## 一、基础通用（所有人必装，不分语言）
### 1. Chinese (Simplified) Language Pack
简体中文汉化，菜单/弹窗/设置全部中文，新手必备，安装重启生效。
### 2. Prettier - Code formatter
**万能格式化**，支持JS/TS/CSS/HTML/Markdown/JSON，开启「保存自动格式化」，统一团队代码风格，搭配ESLint绝配。
### 3. ESLint
JS/TS语法校验，实时标红错误、未定义变量、不规范写法，一键自动修复代码隐患。
### 4. GitLens
Git增强神器，每行代码悬浮看**提交人、修改时间、备注**，支持对比历史、查看分支、回溯代码，不用切命令行。
### 5. Error Lens
直接把报错文字显示在行尾，不用悬浮鼠标，一眼看清代码问题，调试效率翻倍。
### 6. vscode-icons
文件/文件夹彩色图标，区分JS/Vue/Markdown/Docker等文件，目录辨识度大幅提升。
### 7. Indent Rainbow
阶梯彩色缩进，多层代码不看花眼，阅读嵌套逻辑更轻松。
### 8. Better Comments
注释分级着色：普通//、警告//!、TODO//?、重点//*，代码注释一目了然。

## 二、Markdown 专属（贴合你需求，离线写文档）

### 1. Markdown All in One（首选）
一站式MD工具：自动生成目录`[toc]`、表格格式化、快捷键、粘贴自动存图、侧边目录导航、批量格式化MD文件。
### 2. Markdown Preview Enhanced（强推，支持markdown-it底层）
增强预览：Mermaid流程图、PlantUML、LaTeX数学公式、导出PDF/HTML/PNG、分屏同步滚动，底层基于markdown-it解析。
### 3. markdownlint
MD语法规范检查，检测标题层级、空格、链接、表格错误，适合技术文档协作。
### 4. Markdown Inline Editor
类Typora所见即所得，行内直接渲染标题/加粗/图表，不用切预览窗口。

## 三、前端开发（Vue/React/原生）
### 通用前端效率
1. **Live Server**：HTML实时预览，修改自动刷新浏览器，静态页面开发必备。
2. **Auto Rename Tag**：改HTML/JSX开始标签，自动同步闭合标签，杜绝标签不匹配。
3. **Path Intellisense**：文件路径自动补全，导入组件不用手动敲路径。
4. **Code Snap**：一键截取代码截图，支持自定义背景、阴影，写文档/博客截图神器。

### Vue 开发
- Vue Language Features (Volar)：Vue3官方语法提示、类型校验，替代旧Vetur。
- Vue 3 Snippets：快捷生成setup、ref、computed、组件模板。

### React 开发
- ES7+ React Snippets：rfce生成函数组件、useState/useEffect快捷片段。

### CSS/Tailwind
- Tailwind CSS IntelliSense：Tailwind类自动提示、预览、补全。
- CSS Peek：点击class直接跳转对应样式代码。

## 四、AI 编码助手（大幅减少手写代码）
1. **CodeGeeX（国产免费，无需翻墙）**
本地/云端双模式，代码补全、注释生成、翻译代码、解释代码，全语言支持。
2. **GitHub Copilot**
行业标杆AI补全，函数、算法、接口一键生成（付费订阅）。
3. **Cline**
开源本地AI代理，可接入GLM/Claude，自动重构、批量修改文件，全程可控不偷偷改代码。

## 五、后端&运维通用插件
1. **Code Runner**：右键一键运行单文件JS/Python/Go/Shell，无需启动项目。
2. Docker：可视化管理镜像、容器、compose，Dockerfile语法提示。



3. REST Client：内置API测试，写`.http`文件直接发请求，替代Postman。
4. Database Client：MySQL/PostgreSQL/SQLite可视化查询、建表。
5. YAML：配置文件高亮、语法校验（Docker、K8s、GitLab CI）。

## 六、界面美化（解决你之前：纵向Tab改横向、UI自定义）
1. **Customize UI**
核心需求插件：把左侧纵向活动栏（资源管理器图标）移到侧边**顶部/底部横向排列**，隐藏原生竖栏，配置极简。
```json
// settings.json配置示例
"customizeUI.activityBar": "bottom",
"workbench.activityBar.visible": false
```
2. **One Dark Pro**
全球最火深色护眼主题，语法高亮细腻，长时间编码不累眼。
3. Activitus Bar
把侧边功能图标移到**底部状态栏横向**，彻底隐藏左侧竖活动栏。
4. Custom UI Style
自定义VSCode全局CSS，修改字体、背景、标签样式。

## 七、实用工具插件
1. **Translation - 翻译插件**
选中代码/注释一键中英互译，支持驼峰命名翻译。
2. **Regex Previewer**
正则实时预览，写匹配规则不用反复测试。
3. **Draw.io Integration**
MD内嵌入流程图，直接在VSCode绘制架构图。
4. Project Manager：多项目快速切换，一键打开常用文件夹。

## 极简装机方案（新手直接抄）
### 普通写文档/Markdown
Chinese + vscode-icons + One Dark Pro + Markdown All in One + Markdown Preview Enhanced + Customize UI
### 前端开发全套
基础通用8款 + Live Server + Auto Rename Tag + Volar + CodeGeeX
### 程序员全能（前后端+运维+笔记）
上面全部 + Docker + REST Client + Database Client + GitLens