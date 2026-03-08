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

if [ -z "$S3_ACCESS_KEY" ] || [ -z "$S3_SECRET_KEY" ]; then
    echo "Warning: S3_ACCESS_KEY / S3_SECRET_KEY not set, /presign endpoint will return 500"
fi

# 设置默认值
S3_HOST=${S3_HOST:-localhost}
S3_SCHEME=${S3_SCHEME:-http}

# 选择模板
if [ "$NGINX_SSL_MODE" = "1" ]; then
    TEMPLATE="/data/default-ssl.conf.template"
    echo "Mode: SSL (:5000) + HTTP (:5001)"
else
    TEMPLATE="/data/default.conf.template"
    echo "Mode: HTTP (:5001) only"
fi

if [ ! -f "$TEMPLATE" ]; then
    echo "Error: Template not found: $TEMPLATE"
    exit 1
fi

# 从模板生成 nginx 配置文件（使用 | 作为 sed 分隔符，避免凭据中的 / 冲突）
echo "Generating nginx configuration from template..."
sed -e "s|\${NGINX_SERVER_NAME}|$NGINX_SERVER_NAME|g" \
    -e "s|\${S3_ACCESS_KEY}|$S3_ACCESS_KEY|g" \
    -e "s|\${S3_SECRET_KEY}|$S3_SECRET_KEY|g" \
    -e "s|\${S3_HOST}|$S3_HOST|g" \
    -e "s|\${S3_SCHEME}|$S3_SCHEME|g" \
    "$TEMPLATE" > /etc/nginx/conf.d/default.conf

# 显示生成的配置
echo "Generated configuration:"
echo "  Server Name: $NGINX_SERVER_NAME"
echo "  S3 Host:     $S3_HOST"
echo "  S3 Scheme:   $S3_SCHEME"
echo "  S3 Presign:  /presign endpoint enabled"

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
