import { http } from "@/utils/http";

/** 消息类型 */
export interface Message {
  id: number;
  content: string;
  author: string;
  created_at: string;
}

interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

/** 获取消息列表 */
export function getMessages() {
  return http.get<any, ApiResponse<Message[]>>("/api/v1/addons/messages");
}

/** 提交消息 */
export function postMessage(data: { content: string; author: string }) {
  return http.post<ApiResponse<Message>, any>("/api/v1/addons/messages", {
    data
  });
}

/** 删除指定消息 */
export function deleteMessage(id: number) {
  return http.delete<any, ApiResponse<null>>(`/api/v1/addons/messages/${id}`);
}

/** 清空所有消息 */
export function clearMessages() {
  return http.delete<any, ApiResponse<null>>("/api/v1/addons/messages");
}
