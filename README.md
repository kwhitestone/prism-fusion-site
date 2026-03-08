<p align="center">
  <img src="https://raw.githubusercontent.com/kwhitestone/prism-fusion/master/src/admin/public/favicon.svg" width="80" alt="Prism Fusion Site" />
</p>

<h1 align="center">Prism Fusion Site</h1>

<p align="center">
  <strong><a href="https://github.com/kwhitestone/prism-fusion">Prism Fusion</a> 集成站点</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/Vue-3.5-4FC08D?style=flat-square&logo=vue.js" alt="Vue" />
  <img src="https://img.shields.io/badge/Vite-7-646CFF?style=flat-square&logo=vite" alt="Vite" />
  <img src="https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker" alt="Docker" />
  <img src="https://img.shields.io/badge/Casdoor-SSO-purple?style=flat-square" alt="Casdoor" />
  <img src="https://img.shields.io/badge/Casbin-RBAC-orange?style=flat-square" alt="Casbin" />
  <img src="https://img.shields.io/badge/Rclone-S3_Storage-F5BA25?style=flat-square" alt="Rclone S3" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License" />
</p>

---

基于 [Prism Fusion](https://github.com/kwhitestone/prism-fusion) 框架构建的集成站点 —— 通过 git submodule 引用框架，以插件方式集成 Casdoor 单点登录、Casbin 动态权限、Rclone S3 兼容存储，开箱即用。

### 特性

- **Casdoor SSO** — OAuth2 登录 + 自动 Bootstrap（组织 / 应用 / Provider 一键初始化）
- **Casbin RBAC** — 基于策略的细粒度权限控制，支持动态路由下发
- **S3 头像存储** — Rclone 本地磁盘 + Nginx SigV4 代理，兼容任意 S3 后端
- **插件化架构** — 前后端对称的 addons 机制，新增业务只需添加目录
- **多阶段 Docker 构建** — 单条 `docker compose up -d` 拉起全部 6 个服务
- **零手动配置** — 首次启动自动创建 Casdoor 组织 / 应用 / 角色 / 权限 / S3 Bucket

## 项目结构

```
prism-fusion-site/
├── prism-fusion/                  # 框架（git submodule，勿直接修改）
├── app/
│   ├── src/server/                # Go 后端
│   │   ├── addons/                # 业务插件（后端）
│   │   │   ├── casdoor-auth/      #   Casdoor OAuth2 认证 + S3 Bootstrap
│   │   │   ├── casbin-rbac/       #   Casbin RBAC 权限
│   │   │   ├── dashboard/         #   数据总览
│   │   │   ├── messages/          #   消息记录
│   │   │   ├── site-info/         #   站点信息
│   │   │   └── example/           #   示例插件
│   │   ├── go.mod / go.work       # Go workspace（引用框架）
│   │   ├── config.yaml            # 业务配置（覆盖框架默认值）
│   │   └── main.go                # 入口
│   ├── src/admin/                 # Vue 前端
│   │   ├── src/addons/            # 业务插件（前端）
│   │   │   ├── casdoor-auth/      #   Casdoor 登录界面
│   │   │   ├── casbin-rbac/       #   动态路由管理
│   │   │   ├── dashboard/         #   数据总览页面
│   │   │   ├── messages/          #   消息页面
│   │   │   ├── site-info/         #   站点信息页面
│   │   │   └── example/           #   示例插件页面
│   │   ├── pnpm-workspace.yaml    # pnpm workspace（引用框架前端）
│   │   └── package.json
│   └── Dockerfile                 # 多阶段生产构建（Node + Go + Alpine）
├── casdoor/                       # Casdoor IAM 服务
│   ├── casdoor/                   #   Casdoor 源码（git submodule）
│   ├── web_prism/                 #   定制前端（覆盖 Casdoor 默认 UI）
│   ├── casdoor_conf/              #   app.conf 配置
│   ├── nginx/                     #   Casdoor Nginx 代理（会话鉴权 + 预签名转发）
│   └── Dockerfile                 #   Casdoor 多阶段构建
├── rclone/                        # S3 兼容存储层
│   ├── local-storage/             #   本地磁盘存储目录
│   ├── nginx/                     #   S3 Nginx 代理（SigV4 签名 + CORS + 预签名）
│   │   ├── njs/presign.js         #     njs SigV4 签名模块
│   │   └── nginx-conf/            #     Nginx 配置模板
│   └── rclone-admin/              #   Rclone Web GUI 配置（可选）
├── docker-compose.yaml            # 全服务编排（6 个服务）
├── .env.example                   # 环境变量模板
└── README.md
```

## 框架集成方式

本项目通过以下机制引用 Prism Fusion 框架：

| 层 | 机制 | 配置文件 |
|----|------|---------|
| Go 后端 | `go.work` 多模块工作空间 | `app/src/server/go.work` |
| Vue 前端 | pnpm workspace 包引用 | `app/src/admin/pnpm-workspace.yaml` |
| Vite | `@` alias 指向框架 src | `app/src/admin/vite.config.ts` |
| 部署 | git submodule | `.gitmodules` |

业务代码中使用 `@biz/` 别名引用业务模块，`@/` 引用框架模块。

## 服务架构

`docker compose up -d` 启动以下 6 个服务（全部通过 healthcheck + `depends_on: condition: service_healthy` 保证启动顺序）：

```
                   ┌─────────────────────────────────────────────┐
  :3280            │          prism-fusion-site                  │
  (主应用)         │  Go backend + Vue SPA + Supervisor          │
                   └──────┬──────────────────┬──────────────────┘
                          │                  │
                  ┌───────▼───────┐  ┌───────▼────────┐
  :5203           │ casdoor-nginx │  │  rclone-nginx  │  :5208
  (Casdoor 代理)  │ 会话鉴权 +    │  │ SigV4 代理签名 │  (S3 代理)
                  │ 预签名转发    │  │ + CORS + 预签名│
                  └───────┬───────┘  └───────┬────────┘
                          │                  │
                  ┌───────▼───────┐  ┌───────▼────────┐
                  │   casdoor     │  │ rclone-avatar  │
                  │ IAM / SSO    │  │ S3 serve :9000 │
                  └───────┬───────┘  │ (本地磁盘)     │
                          │          └────────────────┘
                  ┌───────▼───────┐
  :3301           │  casdoor-db   │
  (MySQL 调试)    │  MySQL 8.0    │
                  └───────────────┘
```

| 服务 | 容器名 | 宿主机端口 | 说明 |
|------|--------|-----------|------|
| `prism-fusion-site` | prism-fusion-site | `${GATEWAY_PORT:-3280}` | 主应用（Go + Vue） |
| `casdoor-nginx` | prism-casdoor-nginx | `${CASDOOR_NGINX_PORT:-5203}` | Casdoor 反代 + 预签名鉴权 |
| `casdoor` | prism-casdoor | *(仅内网)* | Casdoor 服务（定制前端） |
| `casdoor-db` | prism-casdoor-db | `127.0.0.1:${CASDOOR_DB_PORT:-3301}` | Casdoor MySQL |
| `rclone-nginx` | prism-rclone-nginx | `${RCLONE_NGINX_PORT:-5208}` | S3 SigV4 代理 + 预签名 |
| `rclone-avatar` | prism-rclone-avatar | *(仅内网)* | Rclone S3 serve（本地磁盘） |

> 可选服务：`rclone-admin`（Rclone Web GUI），启动方式：`docker compose --profile admin up -d`

## 前置要求

| 工具 | 版本 |
|------|------|
| Go | >= 1.25 |
| Node.js | >= 22 |
| pnpm | >= 9 |
| Docker + Compose | 部署时需要 |

## 本地开发

### 1. 克隆（含子模块）

```bash
git clone --recurse-submodules https://github.com/kwhitestone/prism-fusion-site.git
cd prism-fusion-site
cp .env.example .env   # 按需修改
```

> 如果已克隆但未拉取子模块：`git submodule update --init --recursive`

### 2. 启动后端

```bash
cd app/src/server
go mod tidy
go run main.go
# 监听 :3280
```

- API 文档 (ReDoc)：http://localhost:3280/redoc
- API 文档 (Scalar)：http://localhost:3280/scalar
- OpenAPI JSON：http://localhost:3280/openapi.json

> 默认使用 SQLite（零配置），数据库文件 `prism_fusion_site.db` 自动创建。

### 3. 启动前端

```bash
cd app/src/admin
pnpm install
pnpm dev
# 访问 http://localhost:3288
```

> Vite 开发服务器已配置代理，`/api` 请求转发到后端 `:3280`。

## 业务插件

本项目在框架内置的 `auth` / `rbac` 插件之上，扩展了以下业务插件：

| 插件 | 说明 | Provider 覆盖 |
|------|------|---------------|
| `casdoor-auth` | Casdoor OAuth2 认证，替换框架内置 JWT 认证 | `auth.provider: casdoor` |
| `casbin-rbac` | Casbin 权限管理，替换框架内置角色管理 | `rbac.provider: casbin` |
| `dashboard` | 数据总览仪表盘 | — |
| `messages` | 消息记录管理 | — |
| `site-info` | 站点信息管理 | — |
| `example` | 插件开发示例 | — |

**添加新插件**只需：

1. 后端：在 `app/src/server/addons/` 下创建插件目录，实现 `Plugin` 接口
2. 在 `app/src/server/addons/addons.go` 中添加 `import _ "whitestone.top/prism-fusion-site/addons/my-plugin"`
3. 前端：在 `app/src/admin/src/addons/` 下创建插件目录，导出 `PluginModule`（自动发现，无需配置）

## 打包部署

### Docker 构建（仅主应用镜像）

```bash
# 从项目根目录执行（多阶段构建：Node 前端 → Go 后端 → Alpine 运行时）
docker build -f app/Dockerfile -t prism-fusion-site .
```

### Docker Compose 一键部署

```bash
cp .env.example .env
# 编辑 .env — 必填项: CASDOOR_ADMIN_PASSWORD, S3_ACCESS_KEY, S3_SECRET_KEY
docker compose up -d
```

启动顺序由 healthcheck 链保证：

```
casdoor-db (healthy) ──→ casdoor (healthy) ──→ prism-fusion-site
rclone-avatar (healthy) ──→ rclone-nginx (healthy) ──↗
```

首次启动时，`casdoor-auth` 插件自动完成以下引导（无需手动操作）：

1. 登录 Casdoor admin 账号
2. 创建 Organization、Application、Captcha Provider
3. 创建 S3 Storage Provider 并绑定到 Application
4. 初始化 S3 bucket，上传默认头像 / Logo
5. 配置角色 & 权限
6. （可选）配置 QQ 邮箱 SMTP Email Provider

> **首次启动耗时较长**（拉取镜像 + 构建 Casdoor 前端），后续启动约 10 秒。

### 验证

```bash
# 检查所有容器状态
docker compose ps

# 查看主应用日志
docker compose logs -f prism-fusion-site
```

- 主应用：http://localhost:3280
- Casdoor 管理台：http://localhost:5203
- S3 文件浏览：http://localhost:5208

## Casdoor 配置

本项目使用 [Casdoor](https://casdoor.org) 作为身份认证服务（通过 `casdoor-auth` 插件集成），默认 **全自动引导**，首次 `docker compose up -d` 后无需手动配置。

### 自动引导（默认）

`.env` 中预填 `CASDOOR_CLIENT_ID` / `CASDOOR_CLIENT_SECRET` 后，插件启动时自动：

- 创建 Organization（`CASDOOR_ORG`）、Application（`CASDOOR_APP`）
- 生成 Captcha Provider、绑定到 Application
- 创建 S3 Storage Provider（对接 Rclone），绑定为 Application 文件存储
- 初始化 S3 bucket `user-avatar`，上传默认头像 & Logo
- 设置 Organization 头像为 S3 中的 Logo
- 创建 Roles & Permissions
- 若 `CASDOOR_QQ_MAIL_SMTP_ACCOUNT` / `CASDOOR_QQ_MAIL_SMTP_SECRET` 非空，自动创建 QQ 邮箱 Email Provider

> Casdoor 管理后台：`http://localhost:${CASDOOR_NGINX_PORT}`，默认账号 `admin / ${CASDOOR_ADMIN_PASSWORD}`

### 外部 Casdoor

如已有 Casdoor 实例，可移除 docker-compose 中的 `casdoor` / `casdoor-db` / `casdoor-nginx` 服务，在 `.env` 中配置：

```env
CASDOOR_ENDPOINT=http://<your-casdoor>:8000
CASDOOR_EXTERNAL_ENDPOINT=https://casdoor.example.com
```

### OIDC 回调地址

如有外部服务需要通过 Casdoor SSO 登录（如 RustFS、AList 等），在 `.env` 中添加回调地址：

```env
EXTERNAL_OIDC_REDIRECT_URIS=https://storage.example.com/callback,https://files.example.com/callback
```

引导时自动注册到 Casdoor Application 的 `redirectUris` 中。

## 环境变量

### 基础配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `GATEWAY_PORT` | `3280` | 应用监听端口 |
| `DB_HOST` | （空，使用 SQLite） | MySQL 主机地址 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_NAME` | `prism_fusion_site` | 数据库名 |
| `DB_USER` | `root` | 数据库用户 |
| `DB_PASSWORD` | — | 数据库密码 |

### Casdoor 认证

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CASDOOR_ENDPOINT` | `http://localhost:5203` | Casdoor 内部地址 |
| `CASDOOR_EXTERNAL_ENDPOINT` | 同 ENDPOINT | Casdoor 外部地址（浏览器跳转） |
| `CASDOOR_CLIENT_ID` | — | Casdoor Client ID（预定义） |
| `CASDOOR_CLIENT_SECRET` | — | Casdoor Client Secret（预定义） |
| `CASDOOR_ORG` | `prism-fusion` | Casdoor 组织名 |
| `CASDOOR_APP` | `app-prism-fusion` | Casdoor 应用名 |
| `CASDOOR_ADMIN_PASSWORD` | — | Casdoor 管理员密码 ⚠️ 必填 |
| `CASDOOR_APP_ADMIN_PASSWORD` | — | 应用组织管理员密码 |

### Casdoor 数据库

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CASDOOR_DB_PASSWORD` | `casdoor_fusion_123` | Casdoor MySQL 密码 |
| `CASDOOR_DB_PORT` | `3301` | 宿主机暴露端口（调试用） |

### Casdoor Nginx

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CASDOOR_NGINX_PORT` | `5203` | 宿主机暴露端口 |
| `CASDOOR_NGINX_SERVER_NAME` | `localhost` | Nginx server_name |
| `STORAGE_PRESIGN_UPSTREAM` | `http://rclone-nginx:5001` | 预签名上游地址 |

### S3 兼容存储（Rclone）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `S3_ENDPOINT` | `http://localhost:5208` | S3 API 地址（应用内部用） |
| `S3_ACCESS_KEY` | — | S3 Access Key ⚠️ 必填 |
| `S3_SECRET_KEY` | — | S3 Secret Key ⚠️ 必填 |
| `S3_PUBLIC_URL` | `http://localhost:5208` | 外部可访问 URL（拼下载地址） |

### Rclone Nginx

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `RCLONE_NGINX_PORT` | `5208` | 宿主机暴露端口 |
| `RCLONE_NGINX_SERVER_NAME` | `localhost` | Nginx server_name |
| `S3_HOST` | `localhost:5208` | 浏览器可达的 S3 地址（预签名用） |
| `S3_SCHEME` | `http` | 预签名 URL scheme（`http` / `https`） |

### Rclone Admin（可选）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `RCLONE_ADMIN_USER` | — | Web GUI 登录用户名 |
| `RCLONE_ADMIN_PASS` | — | Web GUI 登录密码 |
| `RCLONE_ADMIN_PORT` | `5209` | Web GUI 端口 |

### 邮箱（可选）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CASDOOR_QQ_MAIL_SMTP_ACCOUNT` | — | QQ 邮箱 SMTP 账号 |
| `CASDOOR_QQ_MAIL_SMTP_SECRET` | — | QQ 邮箱 SMTP 授权码 |

### 外部 OIDC 回调

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `EXTERNAL_OIDC_REDIRECT_URIS` | — | 外部服务 OIDC 回调（逗号分隔） |

### Nginx SSL 模式

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NGINX_SSL_MODE` | — | 设为 `1` 启用 SSL 双模式（见下方说明） |
| `CORS_ALLOW_ORIGIN` | `https://$NGINX_SERVER_NAME` | （Casdoor 专用）CORS 允许来源 |

完整变量见 [.env.example](.env.example)。

## 外部部署 — Nginx SSL 模式

本项目的 Casdoor Nginx 和 Rclone Nginx 均内置 **SSL 双模式**，适用于通过 frpc / 反向代理将服务暴露到公网的场景。

### 工作原理

每个 Nginx 服务（`casdoor/nginx`、`rclone/nginx`）的 `start-nginx.sh` 检测环境变量 `NGINX_SSL_MODE`：

| `NGINX_SSL_MODE` | 使用模板 | 监听端口 | 适用场景 |
|---|---|---|---|
| 未设置 / 空 | `default.conf.template` | Casdoor: `:8080` / Rclone: `:5001` | 本地开发、Docker 内网 |
| `1` | `default-ssl.conf.template` | **外网** `:5000` (SSL + PROXY protocol) + **内网** Casdoor: `:8080` / Rclone: `:5001` | 公网部署（frpc / 负载均衡） |

SSL 模式下，`:5000` 端口接收 TLS + PROXY protocol v2 连接（通常来自 frpc），`:8080` / `:5001` 保持 HTTP 用于容器间通信。

### 安全增强（SSL 模式）

**Casdoor Nginx** (`:5000`)：
- 屏蔽 built-in 组织：`/api/login`、`/api/signup`、`/login/` 等在 `referer` 包含 `app=app-built-in` 时返回 403
- 启用 CORS：`Access-Control-Allow-Origin` 由 `$CORS_ALLOW_ORIGIN` 控制
- 预签名写入鉴权：`POST /api/upload-resource` 通过 Casdoor session 验证后转发到 Rclone

**Rclone Nginx** (`:5000`)：
- 禁止直接 S3 写入：非预签名的 PUT/POST/DELETE 请求返回 403
- 仅允许预签名写入：URL 带 `X-Amz-Signature` 的请求放行
- 读取（GET/HEAD）不受限制

### 外部 Compose 复用方式

如果您在上层 `docker-compose.yml` 中引用本项目（如通过 frpc 暴露到公网），只需为 Nginx 服务挂载配置目录并设置环境变量，**无需复制或覆盖** Nginx 配置文件。

**Casdoor Nginx 示例：**

```yaml
casdoor-nginx:
  build: ./prism-fusion-site/casdoor/nginx
  volumes:
    - ./prism-fusion-site/casdoor/nginx/nginx-conf:/data:ro
    - /path/to/ssl/certs:/etc/nginx/ssl:ro           # fullchain.cer + *.key
  environment:
    - NGINX_SSL_MODE=1
    - NGINX_SERVER_NAME=casdoor.example.com
    - STORAGE_PRESIGN_UPSTREAM=http://rclone-nginx:5001
    - CORS_ALLOW_ORIGIN=https://app.example.com
  command: ["/bin/sh", "/data/start-nginx.sh"]
```

**Rclone Nginx 示例：**

```yaml
rclone-nginx:
  build: ./prism-fusion-site/rclone/nginx
  volumes:
    - ./prism-fusion-site/rclone/nginx/nginx-conf:/data:ro
    - ./prism-fusion-site/rclone/nginx/njs:/etc/nginx/njs:ro  # njs 签名模块
    - /path/to/ssl/certs:/etc/nginx/ssl:ro
  environment:
    - NGINX_SSL_MODE=1
    - NGINX_SERVER_NAME=storage.example.com
    - S3_ACCESS_KEY=${S3_ACCESS_KEY}
    - S3_SECRET_KEY=${S3_SECRET_KEY}
    - S3_HOST=storage.example.com
    - S3_SCHEME=https
  command: ["/bin/sh", "/data/start-nginx.sh"]
```

**关键要点：**

1. 挂载 `nginx-conf` 目录到 `/data:ro` — 包含模板和启动脚本
2. 挂载 SSL 证书到 `/etc/nginx/ssl:ro` — 需含 `fullchain.cer` 和 `*.key`
3. 设置 `NGINX_SSL_MODE=1` — 启用 SSL 模板
4. Rclone 额外挂载 `njs` 目录 — SigV4 签名模块

### 文件说明

```
casdoor/nginx/nginx-conf/
├── default.conf.template       # HTTP 模式模板（:8080）
├── default-ssl.conf.template   # SSL + HTTP 双模式模板（:5000 + :8080）
└── start-nginx.sh              # 启动脚本（根据 NGINX_SSL_MODE 选择模板）

rclone/nginx/nginx-conf/
├── default.conf.template       # HTTP 模式模板（:5001）
├── default-ssl.conf.template   # SSL + HTTP 双模式模板（:5000 + :5001）
└── start-nginx.sh              # 启动脚本（根据 NGINX_SSL_MODE 选择模板）
```

## 许可证

[MIT License](prism-fusion/LICENSE) © 2025-present [kwhitestone](https://github.com/kwhitestone)
