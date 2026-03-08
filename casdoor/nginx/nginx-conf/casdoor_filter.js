/**
 * njs 脚本：Casdoor 鉴权辅助
 * - authPresign: 头像预签名上传鉴权（校验会话后转发到 storage nginx）
 * - checkBuiltinOrg: 拦截 built-in 组织的登录/注册请求
 */

function checkBuiltinOrg(r) {
    var body = r.requestText || '';
    if (body) {
        try {
            // JSON body (Casdoor 前端默认使用 JSON)
            var data = JSON.parse(body);
            if (data.organization === 'built-in') {
                r.return(403, '{"error":"Access denied: built-in organization login is not allowed from external access"}\n');
                return;
            }
        } catch (e) {
            // URL-encoded fallback: organization=built-in
            if (/(?:^|&)organization=built-in(?:&|$)/.test(body)) {
                r.return(403, '{"error":"Access denied: built-in organization login is not allowed from external access"}\n');
                return;
            }
        }
    }
    r.internalRedirect('@casdoor_backend');
}

/**
 * 头像预签名上传鉴权：先校验 Casdoor 会话，再转发到 storage nginx 获取预签名 URL。
 * 避免未登录用户能为任意用户名生成上传链接。
 */
async function authPresign(r) {
    try {
        // 1. 向 Casdoor 校验会话（通过内部 location 禁用 gzip）
        r.warn('[authPresign] step1: checking auth via /_internal_auth_check');
        let authResp = await r.subrequest('/_internal_auth_check');
        r.warn('[authPresign] step1 done: status=' + authResp.status + ' bodyLen=' + (authResp.responseText || '').length);
        let authData;
        try {
            authData = JSON.parse(authResp.responseText);
        } catch (e) {
            r.warn('[authPresign] step1 parse error: ' + e.message + ' body=' + (authResp.responseText || '').substring(0, 200));
            r.return(502, JSON.stringify({
                code: 1, message: 'auth check: invalid upstream response'
            }));
            return;
        }

        if (authData.status !== 'ok') {
            r.warn('[authPresign] step1 auth failed: status=' + authData.status);
            r.return(401, JSON.stringify({
                code: 1, message: 'unauthorized: please sign in first'
            }));
            return;
        }

        r.warn('[authPresign] step1 auth ok, user=' + (authData.data && authData.data.name || 'unknown'));

        // 2. 鉴权通过，转发到 storage nginx 获取预签名 URL
        r.warn('[authPresign] step2: subrequest /_internal_storage_presign args=' + r.variables.args);
        let presignResp = await r.subrequest('/_internal_storage_presign', {
            args: r.variables.args
        });
        r.warn('[authPresign] step2 done: status=' + presignResp.status + ' body=' + (presignResp.responseText || '').substring(0, 500));

        // 3. 透传 storage 响应
        r.headersOut['Content-Type'] = 'application/json; charset=utf-8';
        r.return(presignResp.status, presignResp.responseText);
    } catch (e) {
        r.warn('[authPresign] exception: ' + e.message + ' stack=' + (e.stack || ''));
        r.return(500, JSON.stringify({
            code: 1, message: 'presign auth error: ' + e.message
        }));
    }
}

export default { checkBuiltinOrg, authPresign };
