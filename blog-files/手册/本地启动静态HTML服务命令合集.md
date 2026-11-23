# 本地启动静态HTML服务命令合集
适用于前端本地预览、挂载HTML静态页面，无需后端代码，一行命令起服务，支持 Python / Node.js / Go / PHP 等

## 一、Python 系列（最常用，电脑预装无需额外安装）
### Python3（推荐，绝大多数系统默认）
1. 最简内置静态服务器（单命令）
```bash
# 进入你的html所在文件夹
cd /你的html文件夹路径
# 启动服务，端口8000
python3 -m http.server 8000
# 访问：http://127.0.0.1:8000
```
2. 指定IP（局域网其他设备访问）
```bash
# 0.0.0.0 允许局域网所有设备连接
python3 -m http.server 8080 --bind 0.0.0.0
```
3. 指定目录启动（不用cd切换文件夹）
```bash
python3 -m http.server 9000 --directory /html文件存放目录
```

### Python2（老旧机器，现已淘汰）
```bash
python -m SimpleHTTPServer 8000
```

## 二、Node.js 系列（前端开发者首选）
### 方式1：http-server（最稳定静态服务工具）
1. 全局安装工具
```bash
npm install -g http-server
# yarn 安装
yarn global add http-server
```
2. 启动命令
```bash
# 进入html目录
cd ./html文件夹
# 默认端口8080
http-server
# 自定义端口 + 允许局域网访问
http-server -p 9090 -a 0.0.0.0
# 关闭缓存（前端调试必备）
http-server -c-1 -p 8080
```

### 方式2：serve（轻量简洁）
```bash
# 安装
npm install -g serve
# 启动当前目录html
serve .
# 指定端口
serve -p 7000 .
```

### 方式3：原生Node.js（无第三方包，内置http模块）
新建 `server.js`，放在html同级目录
```javascript
// server.js
const http = require('http');
const fs = require('fs');
const path = require('path');

const server = http.createServer((req, res) => {
  let filePath = path.join(__dirname, req.url === '/' ? 'index.html' : req.url);
  fs.readFile(filePath, (err, data) => {
    if (err) {
      res.writeHead(404);
      res.end('页面不存在');
      return;
    }
    res.writeHead(200, {'Content-Type': 'text/html'});
    res.end(data);
  });
});
server.listen(3000, '0.0.0.0', () => {
  console.log('服务启动：http://127.0.0.1:3000');
});
```
启动命令：
```bash
node server.js
```

## 三、其他常用简易静态服务（补充）
### 1. PHP（电脑装过PHP环境可用）
```bash
cd html目录
php -S 0.0.0.0:8000
```

### 2. Golang（go环境）
```bash
cd html文件夹
go run -exec="static"
# 或内置文件服务
python3替代更简单，go专用命令：
go serve 8000
```

### 3. Nginx（生产环境，临时本地测试）
临时启动nginx挂载html文件夹（需安装nginx）
```bash
# 切换到html目录启动临时nginx
nginx -p . -c nginx.conf
```
极简nginx.conf配置：
```nginx
server {
    listen 8080;
    location / {
        root ./;
        index index.html;
    }
}
```

# 通用访问说明
1. 本机访问：`http://127.0.0.1:端口`
2. 手机/同局域网电脑访问：`http://本机内网IP:端口`（命令带`0.0.0.0`才支持）
3. 关闭服务：终端按 `Ctrl + C`

# 选型建议
1. 临时快速预览HTML → Python3 一行命令
2. 前端开发、需要禁用缓存、热预览 → Node http-server
3. 生产部署静态页面 → Nginx / Caddy