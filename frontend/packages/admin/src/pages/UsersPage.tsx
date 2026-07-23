import { useEffect, useRef, useState } from 'react';
import { message } from 'antd';
import { ProTable, type ActionType } from '@ant-design/pro-components';
import { authed, getPayload } from '@flash-mall/shared';
import type {
  AdminMutationResp, AdminUserDetailResp, AdminUserItem, AdminUserListResp, AdminUserStatusResp,
} from '@flash-mall/shared';
import UserDetailModal from '../components/users/UserDetailModal';
import { createUserColumns } from '../components/users/userColumns';
import { userMutationError } from '../components/users/userModel';

export default function UsersPage() {
  const actionRef = useRef<ActionType | undefined>(undefined);
  const currentUserId = getPayload()?.user_id || 0;
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<AdminUserItem | null>(null);
  const [initialUserId] = useState(() => {
    const userId = window.__flashAdminUserUserId || 0;
    window.__flashAdminUserUserId = 0;
    return userId;
  });

  const showDetail = async (userId: number) => {
    if (!userId) return;
    setDetailOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const response = await authed<AdminUserDetailResp>(
        `/api/admin/users/detail?user_id=${encodeURIComponent(String(userId))}`,
      );
      const error = userMutationError(response.data as AdminMutationResp);
      if (response.ok && !error) setDetail(response.data);
      else {
        message.error(error || '用户详情加载失败');
        setDetailOpen(false);
      }
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => { if (initialUserId) void showDetail(initialUserId); }, [initialUserId]);

  const updateStatus = async (user: AdminUserItem) => {
    const nextStatus = user.status === 1 ? 2 : 1;
    if (nextStatus === 2 && user.user_id === currentUserId) {
      message.warning('不能禁用当前登录的管理员账号');
      return;
    }
    const response = await authed<AdminUserStatusResp>('/api/admin/users/status', {
      method: 'POST', jsonBody: { user_id: user.user_id, status: nextStatus },
    });
    const error = userMutationError(response.data);
    if (!response.ok || error) {
      message.error(error || '用户状态更新失败');
      return;
    }
    message.success(nextStatus === 1 ? '用户已启用' : '用户已禁用');
    if (detail?.user_id === user.user_id) {
      setDetail({ ...detail, status: response.data.status, status_text: response.data.status_text });
    }
    actionRef.current?.reload();
  };

  return (
    <>
      <ProTable<AdminUserItem>
        actionRef={actionRef}
        columns={createUserColumns(currentUserId, { detail: (id) => void showDetail(id), status: (user) => void updateStatus(user) })}
        rowKey="user_id"
        params={{ initialUserId }}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const query = new URLSearchParams({
            page: String(params.current || 1), page_size: String(params.pageSize || 20),
          });
          const keyword = params.keyword || params.initialUserId;
          if (keyword) query.set('keyword', String(keyword));
          if (params.role) query.set('role', String(params.role));
          if (params.status !== undefined && String(params.status) !== '-1') query.set('status', String(params.status));
          const response = await authed<AdminUserListResp>(`/api/admin/users?${query}`);
          return {
            data: response.ok ? response.data.items || [] : [],
            total: response.ok ? response.data.total : 0,
            success: response.ok,
          };
        }}
        pagination={{ defaultPageSize: 20 }}
      />
      <UserDetailModal
        open={detailOpen}
        loading={detailLoading}
        detail={detail}
        currentUserId={currentUserId}
        onClose={() => setDetailOpen(false)}
        onStatus={(user) => void updateStatus(user)}
      />
    </>
  );
}
