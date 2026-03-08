import type { RouteRecordRaw } from "vue-router";

const Layout = () => import("@/layout/index.vue");

/**
 * Casbin RBAC 管理页面路由
 */
const routes: RouteRecordRaw[] = [
  {
    path: "/rbac",
    name: "RbacManagement",
    component: Layout,
    redirect: "/rbac/users",
    meta: {
      title: "权限管理",
      icon: "ep:lock",
      rank: 10,
      roles: ["role-super-admin"]
    },
    children: [
      {
        path: "/rbac/users",
        name: "RbacUsers",
        component: () => import("../views/users.vue"),
        meta: {
          title: "用户管理",
          icon: "ep:user"
        }
      },
      {
        path: "/rbac/roles",
        name: "RbacRoles",
        component: () => import("../views/roles.vue"),
        meta: {
          title: "角色管理",
          icon: "ep:avatar"
        }
      },
      {
        path: "/rbac/permissions",
        name: "RbacPermissions",
        component: () => import("../views/permissions.vue"),
        meta: {
          title: "权限策略",
          icon: "ep:key"
        }
      }
    ]
  }
];

export default routes;
