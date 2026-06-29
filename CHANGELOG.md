# 更新日志

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
