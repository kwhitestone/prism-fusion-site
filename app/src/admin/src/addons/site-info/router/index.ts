import type { RouteRecordRaw } from "vue-router";

const Layout = () => import("@/layout/index.vue");

/**
 * 站点信息插件路由
 */
const routes: RouteRecordRaw[] = [
  {
    path: "/addon-site-info",
    name: "AddonSiteInfo",
    component: Layout,
    redirect: "/addon-site-info/index",
    meta: {
      title: "站点信息",
      icon: "ep:info-filled",
      rank: 200
    },
    children: [
      {
        path: "/addon-site-info/index",
        name: "AddonSiteInfoIndex",
        component: () => import("../views/index.vue"),
        meta: {
          title: "站点概览",
          icon: "ep:document"
        }
      }
    ]
  }
];

export default routes;
