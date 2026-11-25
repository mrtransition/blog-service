# 轻舟博客（Go 便携版）

一个使用 **Go** 编写的极简本地博客系统，无需数据库，支持 Markdown/HTML 文章，开箱即用。

## 特性
- 纯 Go 实现，编译后仅一个可执行文件
- 自动读取目录结构生成导航
- 支持 Markdown 和 HTML 两种文章格式
- 右侧悬浮文章目录，点击跳转
- 左侧文件名悬浮提示完整名称与绝对路径，并可一键在系统文件管理器中定位
  - 以 Windows 服务方式部署时，服务运行在 Session 0，需通过跨会话投递才能在你的桌面弹出窗口；要求服务以 **LocalSystem** 身份运行（NSSM 默认即是）
- 左右两侧支持收起：左侧导航与右侧文章目录可独立折叠，状态保存在 `localStorage`
- 内容宽度可按百分比调节（50%–100%），选择保存在 `localStorage`
- 全屏阅读：一键隐藏两侧边栏与顶栏，按 `Esc` 或右上角按钮退出
- HTML 文章快速加载：正文直接注入 iframe，剥离外链阻塞样式/脚本并注入 CSP，离线环境也能即时渲染
- 兼容「导出型 HTML」：正文若以 JS 字符串内嵌在 `marked.parse("...")` 中，会自动提取并按 Markdown 渲染，不再白屏
- 代码高亮、图片相对路径自动适配
- 配置文件驱动，博客名称可自定义
- 首次运行自动生成示例配置和文章
- 草稿支持：文件名以 `_` 开头，或 frontmatter 写 `draft: true`
- RSS 订阅：`/feed.xml`
- 深色模式：跟随系统偏好，可手动切换并记住选择
- 元信息快照：启动时扫描一次，`POST /reload` 热刷新
- SEO：`/article/<路径>` 提供 og/canonical 入口，`/sitemap.xml` 仅收录已发布文章
- XSS 防护：Markdown 渲染结果经 DOMPurify 清洗，剥离脚本、事件属性与 `javascript:` URL
- 前端资源本地嵌入：marked、DOMPurify、highlight.js 随二进制分发，无需外部 CDN，离线可用
- 传输压缩与缓存：vendor 资源与页面走 gzip（214KB → 75KB），静态依赖用内容哈希强 ETag 长缓存，弱网首屏约快一倍
- 访问控制：默认仅监听 `127.0.0.1`；可选共享密钥触发 `/reload` 与 Basic Auth

## 快速开始

### 1. 下载可执行文件
从 [Releases]() 页面下载对应系统的 `blog-server` 文件（或自行编译）。

### 2. 运行
```bash
./blog-server -port 3000 -dir blog-files
```

首次运行会自动创建 `config.json` 与 `blog-files/关于.md`。默认只监听 `127.0.0.1`，仅本机可访问。

需要在局域网分享时显式指定监听地址：

```bash
./blog-server -bind 0.0.0.0 -port 3000 \
  -auth-user alice -auth-pass 'change-me' \
  -reload-token 'change-me-too'
```

## 配置项

`config.json` 支持以下字段，命令行同名参数会覆盖配置文件：

| 字段 | 参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `port` | `-port` | `3000` | 监听端口 |
| `bind` | `-bind` | `127.0.0.1` | 监听地址；非回环地址会启用访问控制提示 |
| `blogDir` | `-dir` | `blog-files` | 文章根目录 |
| `blogName` | — | `轻舟博客` | 站点名称 |
| `reloadToken` | `-reload-token` | 空 | 非本机调用 `/reload` 的共享密钥 |
| `authUser` | `-auth-user` | 空 | Basic Auth 用户名，仅非回环监听时生效 |
| `authPass` | `-auth-pass` | 空 | Basic Auth 密码，仅非回环监听时生效 |

## 文章与目录
- 把 `.md` / `.html` 文件放进 `blog-files/` 或其子目录，目录名即分类。
- 图片等资源放在文章同级目录，文章内使用相对路径引用。
- HTML 文章会做离线/性能预处理：移除外链样式与脚本、外链 `@import`，保留内联样式、本地资源与外链图片；因此依赖 CDN 字体或外部 CSS 框架的 HTML 文章会回退到本机字体/基础样式。
- 导航来自启动时的元信息快照；新增、删除或重命名文章后，执行一次：

```bash
curl -X POST http://127.0.0.1:3000/reload
```

默认只接受本机（回环地址）请求，非本机请求返回 403。若部署在外部流水线中，可配置共享密钥后通过请求头触发：

```bash
curl -X POST http://example.com:3000/reload \
  -H 'X-Reload-Token: change-me-too'
# 或 -H 'Authorization: Bearer change-me-too'
```

未配置 `reloadToken` 时，非本机请求一律拒绝。

## 草稿
- 文件名以 `_` 开头（如 `_draft-xxx.md`），或在文件头 frontmatter 写 `draft: true`。
- 草稿不会出现在左侧导航、`/feed.xml` 与 `/sitemap.xml` 中，`/article/<路径>` 也返回 404；但仍可通过 `/#/<路径>` 直接访问预览。
- 去掉下划线 / 删除 `draft: true` 后执行 `/reload`，文章即出现在导航中。

frontmatter 示例（用于 feed 标题、日期与草稿标记）：

```markdown
---
title: 文章标题
date: 2026-11-20
draft: false
---
```

日期缺失时回退为文件修改时间；标题缺失时依次回退为一级标题、文件名。frontmatter 仅用于元信息，不会显示在正文中。

## 订阅、SEO 与主题
- RSS：`/feed.xml`，文章链接使用可被搜索引擎与阅读器识别的 `/article/<路径>`。
- 文章入口：`/article/<路径>` 返回带 `og:title`、`og:description`、`og:url`、`canonical` 与发布时间的 SPA 外壳，浏览器随后规范化为 `/#/<路径>`。
- 站点地图：`/sitemap.xml` 仅列出已发布文章，草稿不会出现。
- 深色模式：默认跟随系统 `prefers-color-scheme`，点击右上角按钮可手动切换，选择保存在 `localStorage`。

## 访问控制
- 默认 `bind` 为 `127.0.0.1`，只有本机可以访问。
- 绑定非回环地址（如 `0.0.0.0`）时，建议同时配置 `authUser`/`authPass` 开启 Basic Auth；未认证请求返回 401。
- `/reload` 不受 Basic Auth 限制，而是单独校验：本机请求直接放行，非本机请求必须携带 `reloadToken`（`X-Reload-Token` 或 `Authorization: Bearer`），否则返回 403。
- `/api/reveal`（在文件管理器中定位文章）只接受**回环地址**的 POST 请求：它会在宿主机桌面上弹出窗口，远程用户即便通过 Basic Auth 也不应能触发。
- **风险提示**：Basic Auth 只做 Base64 编码，HTTP 明文传输下用户名和密码可被网络嗅探。请仅在可信内网使用，或置于 HTTPS 反向代理（Nginx/Caddy 等）之后。

## 安全说明
- 路径读取限制在 `-dir` 根目录内，并校验符号链接，`../` 与编码逃逸请求返回 4xx。
- `/blog-files/` 禁止目录列表。
- `/reload` 默认仅限本机；非回环场景需共享密钥。
- `/api/reveal` 仅限本机调用，且沿用同一套根目录与符号链接校验，不接受越界路径。
- `/api/config` 的 `rootDir`（宿主机绝对路径）只返回给本机请求，绑定局域网时不会泄露目录结构。
- Markdown 渲染结果在写入页面前经 DOMPurify 清洗，`<script>`、事件属性（如 `onerror`）与 `javascript:` URL 会被剥离；前端库随二进制本地嵌入，离线环境也能正常渲染，资源缺失时降级为转义纯文本。
- HTML 文章通过 `iframe sandbox="allow-same-origin"` 隔离，不执行文章内脚本。
