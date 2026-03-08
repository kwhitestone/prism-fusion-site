FROM nginx:alpine
RUN apk add --no-cache nginx-module-njs \
    && sed -i '1i load_module /etc/nginx/modules/ngx_http_js_module.so;' /etc/nginx/nginx.conf
