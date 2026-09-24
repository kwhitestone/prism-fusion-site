import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';

const code = readFileSync(new URL('../nginx-conf/casdoor_filter.js', import.meta.url), 'utf8')
  .replace('export default { checkBuiltinOrg, authPresign };', 'globalThis.authPresign = authPresign;');
const context = vm.createContext({});
vm.runInContext(code, context);
const origin = 'https://site.example.test';
async function request({ incoming = origin, method = 'GET', status = 200, body = '{"status":"ok","data":{"name":"tester"}}', username = 'tester' } = {}) {
  const calls = [], logs = [];
  const r = {
    method, headersIn: { Origin: incoming, Host: 'account.example.test' }, headersOut: {},
    variables: { presign_allowed_origin: origin, scheme: 'https', args: 'username=tester&filename=test.png' },
    args: { username }, warn: s => logs.push(s),
    subrequest: async path => { calls.push(path); return path === '/_internal_auth_check'
      ? { status, responseText: body }
      : { status: 200, responseText: '{"url":"sensitive-capability"}' }; },
    return: (status, body) => { r.result = { status, body }; }
  };
  await context.authPresign(r);
  return { r, calls, logs };
}
test('exact whitelist, original origin retained, credentialed CORS', async () => {
  const { r, calls, logs } = await request();
  assert.equal(r.result.status, 200); assert.equal(calls.length, 2);
  assert.equal(r.headersIn.Origin, origin);
  assert.equal(r.headersOut['Access-Control-Allow-Origin'], origin);
  assert.equal(r.headersOut['Access-Control-Allow-Credentials'], 'true');
  assert.ok(!logs.join().includes('sensitive-capability'));
});
test('lookalike, wildcard and null origins denied before subrequest', async () => {
  for (const incoming of [origin + '.evil.test', origin + '/', 'null', '*', 'https://evil.test']) {
    const { r, calls } = await request({ incoming });
    assert.equal(r.result.status, 403); assert.equal(calls.length, 0);
    assert.equal(r.headersOut['Access-Control-Allow-Origin'], undefined);
  }
});
test('same-origin and server requests still require session check', async () => {
  for (const incoming of ['', 'https://account.example.test']) {
    const { r, calls } = await request({ incoming, body: '{"status":"error"}' });
    assert.equal(r.result.status, 401); assert.equal(calls.length, 1);
  }
});
test('preflight responds locally', async () => {
  const { r, calls } = await request({ method: 'OPTIONS' });
  assert.equal(r.result.status, 204); assert.equal(calls.length, 0);
});
test('empty upstream 401/403 preserved; no JSON parsing or storage request', async () => {
  for (const status of [401, 403]) {
    const { r, calls } = await request({ status, body: '' });
    assert.equal(r.result.status, status); assert.equal(calls.length, 1);
  }
});
test('invalid/unavailable upstream fails closed', async () => {
  for (const options of [{ body: '' }, { status: 500 }, { body: '{"status":"ok"}' }]) {
    const { r, calls } = await request(options);
    assert.ok([401, 502].includes(r.result.status)); assert.equal(calls.length, 1);
  }
});
test('session owner must match requested upload owner', async () => {
  const { r, calls } = await request({ username: 'another-user' });
  assert.equal(r.result.status, 403); assert.equal(calls.length, 1);
});
