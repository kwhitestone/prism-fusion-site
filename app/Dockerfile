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

# ========== 阶段1: Go后端构建阶段 ==========
FROM ${REGISTRY}/golang:1.26.8 AS backend-builder

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

# ========== 阶段2: 最终运行阶段 ==========
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
    mkdir -p /app/data && \
    chown -R app:app /app

# ========== 复制 Go 应用 ==========
COPY --from=backend-builder --chown=app:app /out/prism-example-site /app/prism-fusion
COPY --from=backend-builder --chown=app:app /build/app/src/server/config.yaml /app/

# ========== 配置 Supervisor ==========
COPY prism-fusion/scripts/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# 设置工作目录
WORKDIR /app

# 设置默认端口环境变量
ENV GATEWAY_PORT=3280

# 暴露端口
EXPOSE 3280

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget -qO- http://localhost:${GATEWAY_PORT}/health | grep -q '"status"' || exit 1

# 后端镜像不再执行前端运行时配置脚本
ENTRYPOINT ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
