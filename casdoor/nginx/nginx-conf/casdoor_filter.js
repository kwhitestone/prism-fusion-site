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
    r.headersOut['Content-Type'] = 'application/json; charset=utf-8';
    r.headersOut['Vary'] = 'Origin';
    var origin = r.headersIn['Origin'] || '';
    var allowedOrigin = r.variables.presign_allowed_origin || '';
    var sameOrigin = r.variables.scheme + '://' + r.headersIn['Host'];
    // Exact configured main-site origin or the account site's own origin only.
    // Preserve the incoming Origin on the account-check subrequest: Casdoor
    // must also authorize it. Never clear Origin or substitute a trusted value.
    if (origin && origin !== allowedOrigin && origin !== sameOrigin) {
        r.return(403, JSON.stringify({ code: 1, message: 'origin not allowed' }));
        return;
    }
    if (origin) {
        r.headersOut['Access-Control-Allow-Origin'] = origin;
        r.headersOut['Access-Control-Allow-Credentials'] = 'true';
        r.headersOut['Access-Control-Allow-Methods'] = 'GET, OPTIONS';
        r.headersOut['Access-Control-Allow-Headers'] = 'Content-Type';
    }
    if (r.method === 'OPTIONS') {
        r.return(204);
        return;
    }
    if (r.method !== 'GET') {
        r.return(405, JSON.stringify({ code: 1, message: 'method not allowed' }));
        return;
    }
    try {
        let authResp = await r.subrequest('/_internal_auth_check');
        r.warn('[authPresign] account check status=' + authResp.status);
        if (authResp.status === 401 || authResp.status === 403) {
            r.return(authResp.status, JSON.stringify({ code: 1, message: 'account check denied' }));
            return;
        }
        if (authResp.status !== 200) {
            r.return(502, JSON.stringify({ code: 1, message: 'account check unavailable' }));
            return;
        }
        let authData;
        try {
            authData = JSON.parse(authResp.responseText);
        } catch (e) {
            r.return(502, JSON.stringify({ code: 1, message: 'invalid account check response' }));
            return;
        }
        if (authData.status !== 'ok' || !authData.data || !authData.data.name) {
            r.return(401, JSON.stringify({ code: 1, message: 'unauthorized: please sign in first' }));
            return;
        }
        if (r.args.username !== authData.data.name) {
            r.return(403, JSON.stringify({ code: 1, message: 'upload owner mismatch' }));
            return;
        }
        let presignResp = await r.subrequest('/_internal_storage_presign', { args: r.variables.args });
        // A successful response contains a capability URL; log status only.
        r.warn('[authPresign] storage status=' + presignResp.status);
        r.return(presignResp.status, presignResp.responseText);
    } catch (e) {
        r.warn('[authPresign] subrequest failed');
        r.return(502, JSON.stringify({ code: 1, message: 'presign upstream error' }));
    }
}

export default { checkBuiltinOrg, authPresign };
