#!/bin/sh

# 启动脚本：将环境变量替换到 nginx 配置文件中
# 模式切换：NGINX_SSL_MODE=1 时使用 SSL + HTTP 双模式模板
echo "Initializing Nginx configuration..."

# 检查必需的环境变量
if [ -z "$NGINX_SERVER_NAME" ]; then
    echo "Error: NGINX_SERVER_NAME environment variable is required"
    echo "  NGINX_SERVER_NAME: [NOT SET]"
    exit 1
fi

# 设置默认值
STORAGE_PRESIGN_UPSTREAM=${STORAGE_PRESIGN_UPSTREAM:-http://rclone-nginx:5001}

# 选择模板
if [ "$NGINX_SSL_MODE" = "1" ]; then
    TEMPLATE="/data/default-ssl.conf.template"
    CORS_ALLOW_ORIGIN=${CORS_ALLOW_ORIGIN:-https://$NGINX_SERVER_NAME}
    echo "Mode: SSL (:5000) + HTTP (:8080)"
else
    TEMPLATE="/data/default.conf.template"
    echo "Mode: HTTP (:8080) only"
fi

if [ ! -f "$TEMPLATE" ]; then
    echo "Error: Template not found: $TEMPLATE"
    exit 1
fi

# 从模板生成 nginx 配置文件（使用 | 作为 sed 分隔符，避免 URL 中 / 冲突）
echo "Generating nginx configuration from template..."
sed -e "s|\${NGINX_SERVER_NAME}|$NGINX_SERVER_NAME|g" \
    -e "s|\${STORAGE_PRESIGN_UPSTREAM}|$STORAGE_PRESIGN_UPSTREAM|g" \
    -e "s|\${CORS_ALLOW_ORIGIN}|${CORS_ALLOW_ORIGIN}|g" \
    "$TEMPLATE" > /etc/nginx/conf.d/default.conf

# 显示生成的配置
echo "Generated configuration:"
echo "  Server Name: $NGINX_SERVER_NAME"
echo "  Storage Presign Upstream: $STORAGE_PRESIGN_UPSTREAM"
if [ "$NGINX_SSL_MODE" = "1" ]; then
    echo "  CORS Allow Origin: $CORS_ALLOW_ORIGIN"
fi

# 验证 nginx 配置语法
echo "Testing nginx configuration..."
nginx -t

if [ $? -ne 0 ]; then
    echo "Error: Nginx configuration test failed"
    exit 1
fi

# 启动 nginx
echo "Starting Nginx..."
exec nginx -g "daemon off;"
