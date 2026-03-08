import type { RouteRecordRaw } from "vue-router";

const Layout = () => import("@/layout/index.vue");

/**
 * 示例插件路由配置
 */
const routes: RouteRecordRaw[] = [
  {
    path: "/addon-example",
    name: "AddonExample",
    component: Layout,
    redirect: "/addon-example/index",
    meta: {
      title: "示例插件",
      icon: "ep:box",
      rank: 100 // 菜单排序
    },
    children: [
      {
        path: "/addon-example/index",
        name: "AddonExampleIndex",
        component: () => import("../views/index.vue"),
        meta: {
          title: "示例页面",
          icon: "ep:document"
        }
      }
    ]
  }
];

export default routes;
