const Layout = () => import("@/layout/index.vue");

export default [
  {
    path: "/",
    name: "Prism",
    component: Layout,
    redirect: "/dashboard",
    meta: {
      icon: "ep:data-analysis",
      title: "Prism Fusion",
      rank: 1
    },
    children: [
      {
        path: "/dashboard",
        name: "PrismDashboard",
        component: () => import("../views/index.vue"),
        meta: {
          title: "数据总览",
          showLink: true
        }
      }
    ]
  }
];
