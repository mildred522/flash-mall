import type { AdminMutationResp } from './common';

export interface LoginResp {
  access_token: string;
  refresh_token?: string;
  token_type: string;
  expires_at: number;
  user_id: number;
  display_name: string;
  phone: string;
}

export interface SendCodeResp {
  sent: boolean;
  expires_at: number;
  debug_code?: string;
}

export interface MeResp {
  user_id: number;
  display_name: string;
  phone: string;
  role: string;
}

export interface SecurityEventItem {
  event_type: string;
  result: string;
  user_id: number;
  subject: string;
  ip: string;
  user_agent: string;
  created_at: number;
}

export interface SecurityEventsRecentResp extends AdminMutationResp {
  items: SecurityEventItem[];
}
