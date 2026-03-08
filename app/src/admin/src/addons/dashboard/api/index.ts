import { http } from "@/utils/http";

/** Dashboard 统计数据类型 */
export interface DashboardStats {
  resourceCount: number;
  productCount: number;
  regionCount: number;
  deviceCount: number;
  distributionCount: number;
  syncCount: number;
  downloadCount: number;
}

interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

/** 获取 Dashboard 统计数据 */
export function getDashboardStats() {
  return http.get<any, ApiResponse<DashboardStats>>(
    "/api/v1/addons/dashboard/stats"
  );
}
