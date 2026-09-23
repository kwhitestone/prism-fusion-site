import { resolve } from "node:path";
import { defineConfig, loadEnv } from "vite";
import vue from "@vitejs/plugin-vue";
import vueJsx from "@vitejs/plugin-vue-jsx";
import tailwindcss from "@tailwindcss/vite";
import svgLoader from "vite-svg-loader";
import Icons from "unplugin-icons/vite";

/**
 * Prism Example Site - Vite 配置
 *
 * 通过 pnpm workspace 引用 prism-fusion-web 框架源码，
 * 使用 alias 让框架内部的 @/ 引用正确解析到框架 src 目录。
 */

// 框架源码路径
const frameworkRoot = resolve(__dirname, "../../../prism-fusion/src/web");
const frameworkSrc = resolve(frameworkRoot, "src");
const frameworkBuild = resolve(frameworkRoot, "build");

export default defineConfig(({ mode }) => {
  // 加载业务项目的 env 文件
  const env = loadEnv(mode, __dirname);

  return {
    resolve: {
      alias: {
        // ★ 关键：@ 指向框架 src，使框架内部的 @/ 引用全部正确解析
        "@": frameworkSrc,
        // @build 指向框架 build 目录
        "@build": frameworkBuild,
        // 业务代码用 @biz
        "@biz": resolve(__dirname, "src")
      }
    },
    server: {
      port: Number(env.VITE_PORT) || 3288,
      host: "0.0.0.0",
      proxy: {
        "/api": {
          target: "http://localhost:3280",
          changeOrigin: true,
          ws: true
        }
      },
      fs: {
        // 允许 Vite 访问框架源码目录（pnpm workspace symlink）
        allow: ["../../.."]
      },
      warmup: {
        clientFiles: [
          resolve(frameworkSrc, "views/**/*"),
          resolve(frameworkSrc, "components/**/*")
        ]
      }
    },
    plugins: [
      tailwindcss(),
      vue(),
      vueJsx(),
      svgLoader(),
      Icons({
        compiler: "vue3",
        scale: 1
      })
    ],
    // 不预构建框架 workspace 包，确保 HMR 实时生效
    optimizeDeps: {
      exclude: ["prism-fusion-web"]
    },
    build: {
      target: "es2015",
      sourcemap: false,
      chunkSizeWarningLimit: 4000,
      rollupOptions: {
        input: {
          index: resolve(__dirname, "index.html")
        },
        output: {
          chunkFileNames: "static/js/[name]-[hash].js",
          entryFileNames: "static/js/[name]-[hash].js",
          assetFileNames: "static/[ext]/[name]-[hash].[ext]"
        }
      }
    },
    define: {
      __INTLIFY_PROD_DEVTOOLS__: false,
      __APP_INFO__: JSON.stringify({
        pkg: { name: "prism-example-site-admin", version: "1.0.0" },
        lastBuildTime: new Date().toISOString()
      })
    }
  };
});
