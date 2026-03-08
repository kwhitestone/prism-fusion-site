const Layout = () => import("@/layout/index.vue");

export default [
  {
    path: "/messages",
    name: "Messages",
    component: Layout,
    redirect: "/messages/index",
    meta: {
      icon: "ep:chat-dot-round",
      title: "消息记录",
      rank: 2
    },
    children: [
      {
        path: "/messages/index",
        name: "MessagesList",
        component: () => import("../views/index.vue"),
        meta: {
          title: "消息记录",
          showLink: true
        }
      }
    ]
  }
];
