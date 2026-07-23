export interface SystemHealthResp {
  overall: boolean;
  version: string;
  uptime: string;
  goroutines: number;
  server_time: number;
  dependencies: { name: string; ok: boolean; detail: string }[];
}

export interface ApiResponse<T> {
  ok: boolean;
  status: number;
  data: T;
}

export interface AdminMutationResp {
  ok?: boolean;
  error?: string;
}
