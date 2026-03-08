import { http } from "@/utils/http";

/** 示例项接口响应类型 */
export interface ExampleItem {
  id: number;
  name: string;
  description: string;
  status: number;
  created_at: string;
}

/** API 响应类型 */
interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

/**
 * 获取示例项列表
 */
export function getExampleItems() {
  return http.request<ApiResponse<ExampleItem[]>>(
    "get",
    "/api/v1/addons/example/items"
  );
}

/**
 * 创建示例项
 */
export function createExampleItem(data: {
  name: string;
  description?: string;
}) {
  return http.request<ApiResponse<ExampleItem>>(
    "post",
    "/api/v1/addons/example/items",
    { data }
  );
}

/**
 * 获取示例项详情
 */
export function getExampleItem(id: number) {
  return http.request<ApiResponse<ExampleItem>>(
    "get",
    `/api/v1/addons/example/items/${id}`
  );
}

/**
 * 删除示例项
 */
export function deleteExampleItem(id: number) {
  return http.request<ApiResponse<null>>(
    "delete",
    `/api/v1/addons/example/items/${id}`
  );
}
