# ==================================================
# Prism Example Site - 生产环境多阶段构建
# 基于 prism-fusion 框架 + 业务插件
# ==================================================
#
# 构建命令 (从项目根目录执行):
#   docker build -f app/Dockerfile -t prism-example-site .
#
# ==================================================

# 镜像注册表（可通过构建参数覆盖）
ARG REGISTRY=swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io

# ========== 阶段1: 前端构建阶段 ==========
FROM ${REGISTRY}/node:22.22.0-alpine3.23 AS frontend-builder

WORKDIR /build

ENV ELECTRON_MIRROR=https://npmmirror.com/mirrors/electron/ \
    SASS_BINARY_SITE=https://npmmirror.com/mirrors/node-sass/ \
    PHANTOMJS_CDNURL=https://npmmirror.com/mirrors/phantomjs/ \
    CHROMEDRIVER_CDNURL=https://npmmirror.com/mirrors/chromedriver/ \
    PYTHON_MIRROR=https://npmmirror.com/mirrors/python/

RUN npm config set registry https://registry.npmmirror.com/ && \
    npm config set cache /tmp/.npm && \
    npm install -g pnpm && \
    pnpm config set registry https://registry.npmmirror.com/

# 复制 pnpm workspace 配置
COPY app/src/admin/pnpm-workspace.yaml /build/app/src/admin/
COPY app/src/admin/.npmrc /build/app/src/admin/
COPY app/src/admin/package.json app/src/admin/pnpm-lock.yaml /build/app/src/admin/

# 复制框架前端 package.json（workspace 需要）
COPY prism-fusion/src/admin/package.json /build/prism-fusion/src/admin/

# 安装依赖（lockfile 在 workspace root: app/src/admin/）
WORKDIR /build/app/src/admin
RUN pnpm install --frozen-lockfile

# 复制框架前端源码
COPY prism-fusion/src/admin/ /build/prism-fusion/src/admin/

# 复制业务前端源码
COPY app/src/admin/ /build/app/src/admin/

# 构建
RUN pnpm run build

# ========== 阶段2: Go后端构建阶段 ==========
FROM ${REGISTRY}/golang:1.25.5 AS backend-builder

ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Set non-interactive mode to avoid tzdata blocking
ENV DEBIAN_FRONTEND=noninteractive

# 使用中国镜像加速 apt
RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources

RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

# 复制 go.work
COPY app/src/server/go.work /build/app/src/server/
COPY app/src/server/go.mod app/src/server/go.sum /build/app/src/server/
COPY prism-fusion/src/server/go.mod prism-fusion/src/server/go.sum /build/prism-fusion/src/server/

# 下载依赖
WORKDIR /build/app/src/server
RUN go mod download && go mod verify

# 复制所有源码
COPY prism-fusion/src/server/ /build/prism-fusion/src/server/
COPY app/src/server/ /build/app/src/server/

# 构建
RUN go build -ldflags="-s -w -extldflags '-static'" -a -installsuffix cgo -o /out/prism-example-site ./main.go

# ========== 阶段3: 最终运行阶段 ==========
FROM ${REGISTRY}/library/alpine:3.23 AS final

# 设置时区
ENV TZ=Asia/Shanghai

# 更新镜像源并安装依赖
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk update && \
    apk add --no-cache \
        ca-certificates \
        tzdata \
        bash \
        supervisor \
        wget \
        netcat-openbsd \
        jq \
        curl && \
    ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
    echo ${TZ} > /etc/timezone && \
    rm -rf /var/cache/apk/*

# 创建应用用户和目录
RUN addgroup -S app && adduser -S app -G app && \
    mkdir -p /app/web /app/data && \
    chown -R app:app /app

# ========== 复制 Go 应用和前端资源 ==========
COPY --from=backend-builder --chown=app:app /out/prism-example-site /app/prism-fusion
COPY --from=backend-builder --chown=app:app /build/app/src/server/config.yaml /app/
COPY --from=frontend-builder --chown=app:app /build/app/src/admin/dist/ /app/web/

# ========== 配置 Supervisor ==========
COPY prism-fusion/scripts/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# ========== 复制启动脚本 ==========
COPY --chown=app:app prism-fusion/scripts/entrypoint.sh /app/entrypoint.sh

# 设置工作目录
WORKDIR /app

# 设置默认端口环境变量
ENV GATEWAY_PORT=8080

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget -qO- http://localhost:${GATEWAY_PORT}/health | grep -q '"status"' || exit 1

# 使用 entrypoint 脚本启动
ENTRYPOINT ["/app/entrypoint.sh"]
