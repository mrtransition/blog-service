# 更新日志

## [1.3.0] - 2026-11-23

### 安全
- Markdown 渲染接入 DOMPurify：剥离 `<script>`、事件属性与 `javascript:` URL；清洗库未加载时降级为转义纯文本
- HTML 文章 iframe 移除 `allow-scripts`，文章内脚本不再执行
- 默认监听地址改为 `127.0.0.1`，避免无意暴露到局域网/公网
- `/reload` 在非回环场景下必须携带共享密钥（`X-Reload-Token` 或 `Authorization: Bearer`），未配置密钥时外部请求一律 403
- 新增可选 Basic Auth（`-auth-user` / `-auth-pass`），仅在绑定非回环地址时生效；`/reload` 使用独立密钥校验

### 新功能
- SEO 文章入口 `/article/<路径>`：注入 `og:title`、`og:description`、`og:url`、`canonical` 与发布时间，草稿返回 404
- 新增 `/sitemap.xml`，仅收录已发布文章
- RSS 与 sitemap 中的文章链接统一为 `/article/<编码路径>`
- 新增 `-bind`、`-reload-token`、`-auth-user`、`-auth-pass` 命令行参数及同名配置字段
- 启动日志在非回环监听、未配置密钥/认证时给出明确警告

### 修复
- 前端依赖改为本地嵌入（marked 18.2.1 / DOMPurify 3.5.0 / highlight.js 11.9.0），修复 CDN 不可达时 Markdown 退化为源码的问题
- Markdown 正文渲染前剥离文件头部的 YAML frontmatter，避免 `---` 元数据块重复显示；标题/日期/草稿标记仍由服务端解析
- 清洗库不可用时，降级文本同样不再包含 frontmatter

### 测试
- 新增文章页 og/canonical、草稿 404、sitemap 过滤、reload 共享密钥、Basic Auth 401/200/默认关闭、`isLoopbackBind` 表驱动测试
- 浏览器端验证：恶意 Markdown 中的 `onerror`、`javascript:` 链接与 `<script>` 均被移除，正常加粗/标题渲染不变，frontmatter 不再显示在正文中

## [1.2.0] - 2026-11-20

### 安全
- 路径遍历防护：`/api/file` 与 `/blog-files/` 统一走 `filepath.IsLocal` + `EvalSymlinks` 根目录校验，拒绝 `../`、绝对路径与指向根目录外的符号链接
- 新增全局 URL 路径守卫，编码后的 `%2e%2e`、`%5c`、`%00` 在进入路由前直接返回 400
- `/blog-files/` 禁用目录列表，并补充 `X-Content-Type-Options: nosniff`
- `/reload` 仅允许回环地址访问，且只接受 POST

### 修复
- `index.html` 由 `go:embed` 提供时补充内容哈希 ETag 与 `Cache-Control: no-cache`，支持 `If-None-Match` 返回 304，替换二进制后 ETag 自动变化

### 新功能
- 草稿：文件名以 `_` 开头或 frontmatter `draft: true`，从导航与 feed 过滤，但仍可直接访问预览
- RSS 2.0：新增 `/feed.xml`，标题/日期取自 frontmatter，缺失时回退一级标题与文件 mtime
- 深色模式：跟随系统偏好，右上角按钮手动切换并持久化到 `localStorage`，同步切换 highlight.js 主题
- `POST /reload`：仅本机可触发的元信息快照热刷新

### 性能
- 启动时一次性 `WalkDir` 生成 `{path, mtime, title, category, isDraft}` 元信息快照，导航与 feed 读取不可变快照，不再逐请求递归扫描目录

### 测试
- 新增表驱动测试：路径逃逸/符号链接逃逸、URL 守卫、ETag/304、草稿过滤、feed 内容与 XML 结构、reload 回环限制

## [1.1.0] - 2026-06-29

### 修复
- 修复左侧搜索框无效：补充缺失的 CSS 类 `.hidden-by-search`，搜索结果可正确隐藏节点
- 修复目录层级 2 级及以上显示异常：
  - `updateChildrenHeight` 测量前临时解除后代 `.children` 的 `max-height` 约束，确保 `scrollHeight` 计算正确
  - 新增 `updateTreeHeight` 递归更新祖先高度，展开深层目录时父级高度同步更新
- 修复 CSS 选择器注入漏洞：`highlightActiveNode` 改用迭代比对 `dataset`，移除 `querySelector` 拼接
- 修复重复初始化：移除冗余的 `loadTree()` 调用

### 交互优化
- 面包屑中间路径节点点击后展开并滚动到对应目录
- 搜索无结果时显示"未找到匹配的文章"空状态提示
- Hash 路由加载时跳过欢迎页渲染，消除页面闪烁

### UI 改进
- 目录图标从 Emoji 替换为 SVG 矢量图标，保证跨平台一致
- `.top-bar` 毛玻璃效果添加 `@supports` 降级方案
- 移动端遮罩层改用 `opacity` + `pointer-events` 控制显隐，修复过渡动画失效
- 移除 `.article-card` 不必要的 hover 阴影

### 后端
- 添加请求日志中间件，输出请求方法与路径
- 空目录现在会在目录树中显示（此前被过滤不可见）

### 性能
- 移除 8 个未使用的 Highlight.js 语言包（保留 javascript、bash）
