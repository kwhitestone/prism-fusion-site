/**
 * presign.js — AWS SigV4 Presigned PUT URL 生成器 (njs)
 *
 * 为浏览器直传 S3 生成限时预签名 URL（适用于 rclone / RustFS / MinIO 等任意 S3 后端）。
 * AK/SK 只存在于服务端 nginx 变量中，绝不暴露给前端。
 *
 * nginx 配置示例：
 *   load_module /etc/nginx/modules/ngx_http_js_module.so;   (main context)
 *   js_import presign from /etc/nginx/njs/presign.js;        (http context)
 *
 *   location = /presign {
 *       set $s3_access_key '...';
 *       set $s3_secret_key '...';
 *       set $s3_host 'localhost:5208';
 *       set $s3_scheme 'http';
 *       js_content presign.handle;
 *   }
 */

var REGION  = 'us-east-1';
var SERVICE = 's3';
var BUCKET  = 'user-avatar';
var EXPIRY  = 60;               // 预签名有效期（秒）

// ---- 加密辅助 ----

function hmac(key, msg) {
    return require('crypto').createHmac('sha256', key).update(msg).digest();
}

function hmacHex(key, msg) {
    return require('crypto').createHmac('sha256', key).update(msg).digest('hex');
}

function sha256Hex(msg) {
    return require('crypto').createHash('sha256').update(msg).digest('hex');
}

// AWS SigV4 signing key 派生
function deriveSigningKey(secret, dateStamp) {
    var kDate    = hmac('AWS4' + secret, dateStamp);
    var kRegion  = hmac(kDate, REGION);
    var kService = hmac(kRegion, SERVICE);
    return hmac(kService, 'aws4_request');
}

// ---- 日期格式 ----

function pad(n) { return n < 10 ? '0' + n : String(n); }

function amzTimestamp() {
    var d  = new Date();
    var ds = '' + d.getUTCFullYear() +
             pad(d.getUTCMonth() + 1) +
             pad(d.getUTCDate());
    return {
        full: ds + 'T' + pad(d.getUTCHours()) +
                         pad(d.getUTCMinutes()) +
                         pad(d.getUTCSeconds()) + 'Z',
        date: ds
    };
}

// RFC 3986 URI 编码（比 encodeURIComponent 更严格）
function encodeRfc3986(str) {
    return encodeURIComponent(str).replace(/[!'()*]/g, function (c) {
        return '%' + c.charCodeAt(0).toString(16).toUpperCase();
    });
}

// ---- CORS 工具 ----

function setCorsHeaders(r) {
    var origin = r.headersIn['Origin'] || '';
    if (origin) {
        r.headersOut['Access-Control-Allow-Origin'] = origin;
    }
    r.headersOut['Access-Control-Allow-Methods'] = 'GET, OPTIONS';
    r.headersOut['Access-Control-Allow-Headers'] = 'Content-Type';
}

// ---- 核心入口 ----

function handle(r) {
    // CORS preflight
    if (r.method === 'OPTIONS') {
        var origin = r.headersIn['Origin'] || '*';
        r.headersOut['Access-Control-Allow-Origin']  = origin;
        r.headersOut['Access-Control-Allow-Methods']  = 'GET, OPTIONS';
        r.headersOut['Access-Control-Allow-Headers']  = 'Content-Type';
        r.headersOut['Access-Control-Max-Age']         = '3600';
        r.return(204);
        return;
    }

    // 设置 CORS + JSON content-type
    setCorsHeaders(r);
    r.headersOut['Content-Type'] = 'application/json; charset=utf-8';

    // ---- 参数校验 ----
    var username = r.args.username;
    var filename = r.args.filename;
    if (!username || !filename) {
        r.return(400, JSON.stringify({
            code: 1, message: 'username and filename are required'
        }));
        return;
    }

    // 用户名格式校验：防止路径穿越（仅允许字母、数字、下划线、连字符、点号）
    if (!/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$/.test(username)) {
        r.return(400, JSON.stringify({
            code: 1, message: 'invalid username format'
        }));
        return;
    }

    // 文件扩展名白名单
    var ALLOWED_EXT = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.svg'];
    var dotIdx  = filename.lastIndexOf('.');
    var ext     = dotIdx > 0 ? filename.substring(dotIdx).toLowerCase() : '';
    if (ALLOWED_EXT.indexOf(ext) === -1) {
        r.return(400, JSON.stringify({
            code: 1, message: 'unsupported file extension, allowed: ' + ALLOWED_EXT.join(', ')
        }));
        return;
    }

    var accessKey = r.variables.s3_access_key;
    var secretKey = r.variables.s3_secret_key;
    var s3Host    = r.variables.s3_host;
    var s3Scheme  = r.variables.s3_scheme || 'https';
    if (!accessKey || !secretKey || !s3Host) {
        r.return(500, JSON.stringify({
            code: 1, message: 'S3 credentials not configured'
        }));
        return;
    }

    // ---- 构造 object key：avatar/{username}/avatar.{ext} ----
    var objectKey = 'avatar/' + username + '/avatar' + ext;

    // ---- AWS SigV4 Presigned URL ----
    var dt         = amzTimestamp();
    var credential = accessKey + '/' + dt.date + '/' + REGION + '/' +
                     SERVICE + '/aws4_request';

    // 查询参数（按 key 字典序排列）
    var qs = 'X-Amz-Algorithm=AWS4-HMAC-SHA256' +
             '&X-Amz-Credential=' + encodeRfc3986(credential) +
             '&X-Amz-Date='       + dt.full +
             '&X-Amz-Expires='    + EXPIRY +
             '&X-Amz-SignedHeaders=host';

    // 规范 URI（路径式访问，每段单独编码）
    var canonicalUri = '/' + BUCKET + '/' +
        objectKey.split('/').map(encodeRfc3986).join('/');

    // 规范请求
    var canonicalRequest =
        'PUT\n' +
        canonicalUri + '\n' +
        qs + '\n' +
        'host:' + s3Host + '\n' +
        '\n' +
        'host\n' +
        'UNSIGNED-PAYLOAD';

    // 待签字符串
    var scope = dt.date + '/' + REGION + '/' + SERVICE + '/aws4_request';
    var stringToSign =
        'AWS4-HMAC-SHA256\n' +
        dt.full + '\n' +
        scope + '\n' +
        sha256Hex(canonicalRequest);

    // 签名
    var sk        = deriveSigningKey(secretKey, dt.date);
    var signature = hmacHex(sk, stringToSign);

    // 拼接最终 URL（scheme 通过 nginx 变量配置，本地 http / 生产 https）
    var presignedUrl = s3Scheme + '://' + s3Host + canonicalUri +
        '?' + qs + '&X-Amz-Signature=' + signature;
    var avatarUrl = s3Scheme + '://' + s3Host + '/' + BUCKET + '/' + objectKey;

    r.return(200, JSON.stringify({
        code: 0,
        message: 'success',
        data: {
            presignedUrl: presignedUrl,
            avatarUrl:    avatarUrl
        }
    }));
}

// ---- 代理签名：nginx 代理到 rclone 时自动添加 SigV4 鉴权 ----
//
// rclone serve s3 启用 --auth-key 后，所有请求需 S3 签名。
// nginx 在代理时统一签名（AK/SK 仅存在于 nginx 变量中）。
//
// nginx 配置：
//   js_set $s3_proxy_auth presign.proxyAuth;          (http context)
//
//   server {
//       set $s3_access_key '${S3_ACCESS_KEY}';
//       set $s3_secret_key '${S3_SECRET_KEY}';
//       set $s3_proxy_date '';                         (必须声明可写变量)
//
//       location ... {
//           proxy_set_header Authorization $s3_proxy_auth;  (必须先于 x-amz-date)
//           proxy_set_header x-amz-date $s3_proxy_date;
//           proxy_set_header x-amz-content-sha256 "UNSIGNED-PAYLOAD";
//           proxy_pass http://rclone-...:9000$uri;
//       }
//   }

function proxyAuth(r) {
    var ak = r.variables.s3_access_key;
    var sk = r.variables.s3_secret_key;
    if (!ak || !sk) {
        return '';
    }

    var dt = amzTimestamp();
    // 写入 nginx 可写变量，供 proxy_set_header x-amz-date 读取
    r.variables.s3_proxy_date = dt.full;

    var host        = r.headersIn['Host'] || 'localhost';
    var payloadHash = 'UNSIGNED-PAYLOAD';

    // URI 编码各路径段（SigV4 规范）
    var canonicalUri = r.uri.split('/').map(function (seg) {
        return seg ? encodeRfc3986(seg) : '';
    }).join('/') || '/';

    // Canonical request（query string 始终为空 — proxy_pass $uri 已剥离）
    var canonicalHeaders =
        'host:' + host + '\n' +
        'x-amz-content-sha256:' + payloadHash + '\n' +
        'x-amz-date:' + dt.full + '\n';
    var signedHeaders = 'host;x-amz-content-sha256;x-amz-date';

    var canonicalRequest =
        r.method + '\n' +
        canonicalUri + '\n' +
        '\n' +
        canonicalHeaders + '\n' +
        signedHeaders + '\n' +
        payloadHash;

    // String to sign
    var scope = dt.date + '/' + REGION + '/' + SERVICE + '/aws4_request';
    var stringToSign =
        'AWS4-HMAC-SHA256\n' +
        dt.full + '\n' +
        scope + '\n' +
        sha256Hex(canonicalRequest);

    // Signature
    var signingKey = deriveSigningKey(sk, dt.date);
    var signature  = hmacHex(signingKey, stringToSign);

    return 'AWS4-HMAC-SHA256 Credential=' + ak + '/' + scope +
        ', SignedHeaders=' + signedHeaders + ', Signature=' + signature;
}

export default { handle, proxyAuth };
