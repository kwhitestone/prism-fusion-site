import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { runInNewContext } from "node:vm";
import { compileScript, parse } from "@vue/compiler-sfc";
import ts from "typescript";

// Execute the real addons, replacing only browser, store and HTTP boundaries.
function fixture() {
  let token = {
    sessionId: "session",
    refreshToken: "refresh",
    refreshRequestId: "request"
  };
  let observed;
  let writes = 0;
  let epoch = 0;
  let sequence = 0;
  let mounted;
  let unmounted;
  let locked = false;
  let refreshResponse = async () => response();
  let routesResponse = async () => ({ data: { success: true, data: [] } });
  const listeners = new Map();
  const slots = {};
  const slot = key => value => {
    const previous = slots[key];
    slots[key] = value;
    return () => {
      slots[key] = previous;
    };
  };
  const response = () => ({
    data: {
      code: 0,
      data: {
        accessToken: "access",
        refreshToken: "rotated",
        expiresIn: 360,
        user: { username: "alice" }
      }
    }
  });
  const dependencies = {
    getConfig: () => ({ AuthProvider: "casdoor", RBACProvider: "casbin" }),
    setRefreshHandler: slot("refresh"),
    setUserInfoHandler: slot("userInfo"),
    setLoginComponent: slot("login"),
    setAsyncRoutesProvider: slot("routes"),
    useUserStoreHook: () => ({
      endSession() {},
      async fetchUserInfo() {
        assert.equal(observed, token.sessionId);
      }
    }),
    getToken: () => token,
    setToken: value => {
      token = value;
      writes++;
    },
    setAuthToken() {},
    userKey: "user-info",
    endAuthSessionIfCurrent: async id => {
      if (token?.sessionId === id) token = undefined;
    },
    createAuthRequestID: () => `id-${++sequence}`,
    currentAuthSessionEpoch: () => epoch,
    isAuthSessionEpoch: value => value === epoch,
    invalidateAuthSession: () => {
      epoch++;
    },
    observeAuthSession: id => {
      observed = id;
    },
    getObservedAuthSession: () => ({ sessionId: observed }),
    crossTabSessionAction: () => "none",
    withAuthSessionLock: async operation => {
      locked = true;
      try {
        return await operation();
      } finally {
        locked = false;
      }
    },
    refreshToken: () => refreshResponse(),
    getAsyncRoutes: () => routesResponse(),
    getUserInfo: async () => ({
      data: {
        code: 0,
        data: { username: "alice", nickName: "Alice", roles: ["user"] }
      }
    }),
    signinCallback: async () => response(),
    getSigninUrl: async () => ({ data: { data: { url: "/casdoor" } } }),
    ref: value => ({ value }),
    onMounted: callback => {
      mounted = callback;
    },
    onUnmounted: callback => {
      unmounted = callback;
    },
    defineComponent: value => value,
    useRouter: () => ({ push: async () => {} }),
    useRoute: () => ({ query: { code: "test-code" } }),
    initRouter: async () => {
      assert.equal(locked, false);
    },
    getTopMenu: () => ({ path: "/dashboard/index" }),
    triggerPluginRegistryReport() {},
    message() {}
  };
  function load(relativePath, component = false) {
    let source = readFileSync(
      new URL(`../src/addons/${relativePath}`, import.meta.url),
      "utf8"
    );
    if (component)
      source = compileScript(parse(source).descriptor, {
        id: "test-login"
      }).content;
    const exports = {};
    runInNewContext(
      ts.transpileModule(source, {
        compilerOptions: {
          module: ts.ModuleKind.CommonJS,
          target: ts.ScriptTarget.ES2022
        }
      }).outputText,
      {
        exports,
        require: () => dependencies,
        console: { log() {}, warn() {} },
        URLSearchParams,
        window: {
          addEventListener: (type, callback) => listeners.set(type, callback),
          removeEventListener: type => listeners.delete(type),
          location: {
            origin: "https://site.test",
            pathname: "/login",
            search: "",
            hash: "",
            reload() {}
          },
          history: { replaceState() {} }
        }
      }
    );
    return exports.default;
  }
  return {
    load,
    slots,
    listeners,
    dependencies,
    response,
    state: () => ({ token, observed, writes }),
    refresh: callback => {
      refreshResponse = callback;
    },
    routes: callback => {
      routesResponse = callback;
    },
    mounted: () => mounted(),
    unmount: () => unmounted()
  };
}

test("Casdoor refresh preserves the session, rotates request IDs and restores providers on disposal", async () => {
  const f = fixture();
  const old = async () => ({ success: false });
  f.slots.refresh = old;
  const auth = f.load("casdoor-auth/index.ts");
  assert.equal(auth.enabled(), true);
  auth.setup();
  assert.equal(f.listeners.size, 1);
  assert.equal((await f.slots.userInfo()).data.nickname, "Alice");
  const result = await f.slots.refresh({
    sessionId: "session",
    refreshToken: "refresh",
    requestId: "request"
  });
  assert.equal(result.success, true);
  assert.equal(f.state().token.sessionId, "session");
  assert.equal(f.state().token.refreshToken, "rotated");
  assert.notEqual(f.state().token.refreshRequestId, "request");
  assert.equal(f.state().observed, "session");
  auth.destroy();
  auth.destroy();
  assert.equal(f.listeners.size, 0);
  assert.equal(f.slots.refresh, old);
});

test("refresh cannot overwrite a replaced session or write after disposal", async () => {
  for (const change of ["session", "dispose"]) {
    const f = fixture();
    const auth = f.load("casdoor-auth/index.ts");
    auth.setup();
    let finish;
    f.refresh(
      () =>
        new Promise(resolve => {
          finish = resolve;
        })
    );
    const pending = f.slots.refresh({
      sessionId: "session",
      refreshToken: "refresh",
      requestId: "request"
    });
    if (change === "session")
      f.dependencies.setToken({ sessionId: "new-session" });
    else auth.destroy();
    const writes = f.state().writes;
    finish(f.response());
    assert.equal((await pending).sessionChanged, true);
    assert.equal(f.state().writes, writes);
    auth.destroy();
  }
});

test("legacy browser sessions are cleared during provider activation", () => {
  const f = fixture();
  f.dependencies.setToken({ refreshToken: "legacy" });
  const auth = f.load("casdoor-auth/index.ts");
  auth.setup();
  assert.equal(f.state().token, undefined);
  auth.destroy();
});

test("Casbin disposal restores the previous route provider and discards pending routes", async () => {
  const f = fixture();
  const prior = async () => ({ success: true, data: [] });
  f.slots.routes = prior;
  const rbac = f.load("casbin-rbac/index.ts");
  assert.equal(rbac.enabled(), true);
  assert.equal(rbac.manifest.requires[0].id, "casdoor-auth");
  rbac.setup();
  let finish;
  f.routes(
    () =>
      new Promise(resolve => {
        finish = resolve;
      })
  );
  const pending = f.slots.routes();
  rbac.destroy();
  finish({ data: { success: true, data: [{ path: "/stale" }] } });
  assert.equal((await pending).success, false);
  assert.equal(f.slots.routes, prior);
});

test("OAuth callback commits V2 identity before fetching user info and routing", async () => {
  const f = fixture();
  const login = f.load("casdoor-auth/components/CasdoorLogin.vue", true);
  login.setup({}, { expose() {} });
  await f.mounted();
  assert.equal(f.state().writes, 1);
  assert.ok(f.state().token.sessionId);
  assert.ok(f.state().token.refreshRequestId);
  assert.equal(f.state().observed, f.state().token.sessionId);
});

test("an unmounted OAuth callback cannot commit late credentials", async () => {
  const f = fixture();
  let finish;
  f.dependencies.signinCallback = () =>
    new Promise(resolve => {
      finish = resolve;
    });
  f.load("casdoor-auth/components/CasdoorLogin.vue", true).setup(
    {},
    { expose() {} }
  );
  const pending = f.mounted();
  f.unmount();
  finish(f.response());
  await pending;
  assert.equal(f.state().writes, 0);
});
