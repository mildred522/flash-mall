import { useEffect, useRef, useState } from 'react';
import { Button, Descriptions, Modal, Popconfirm, Space, Tag, message } from 'antd';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed, getPayload } from '@flash-mall/shared';
import type { AdminMutationResp, AdminUserDetailResp, AdminUserItem, AdminUserListResp, AdminUserStatusResp } from '@flash-mall/shared';

function mutationError(data: AdminMutationResp): string {
  if (data.error === 'cannot disable current admin') {
    return '不能禁用当前登录的管理员账号';
  }
  return data.error || '';
}

function userStatusTag(user: Pick<AdminUserItem, 'status'>) {
  return user.status === 2 ? <Tag color="red">禁用</Tag> : <Tag color="green">启用</Tag>;
}

function userRoleTag(user: Pick<AdminUserItem, 'role'>) {
  return user.role === 'admin' ? <Tag color="red">管理员</Tag> : <Tag color="blue">用户</Tag>;
}

export default function UsersPage() {
  const actionRef = useRef<ActionType>();
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
      const res = await authed<AdminUserDetailResp>(`/api/admin/users/detail?user_id=${encodeURIComponent(String(userId))}`);
      const error = mutationError(res.data as AdminMutationResp);
      if (res.ok && !error) {
        setDetail(res.data);
      } else {
        message.error(error || '用户详情加载失败');
        setDetailOpen(false);
      }
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    if (initialUserId) {
      void showDetail(initialUserId);
    }
  }, [initialUserId]);

  const updateStatus = async (user: AdminUserItem) => {
    const nextStatus = user.status === 1 ? 2 : 1;
    if (nextStatus === 2 && user.user_id === currentUserId) {
      message.warning('不能禁用当前登录的管理员账号');
      return;
    }

    const res = await authed<AdminUserStatusResp>('/api/admin/users/status', {
      method: 'POST',
      jsonBody: { user_id: user.user_id, status: nextStatus },
    });
    const error = mutationError(res.data);
    if (res.ok && !error) {
      message.success(nextStatus === 1 ? '用户已启用' : '用户已禁用');
      if (detail?.user_id === user.user_id) {
        setDetail({ ...detail, status: res.data.status, status_text: res.data.status_text });
      }
      actionRef.current?.reload();
    } else {
      message.error(error || '用户状态更新失败');
    }
  };

  const columns: ProColumns<AdminUserItem>[] = [
    { title: '用户ID', dataIndex: 'user_id', width: 100, search: false },
    { title: '关键词', dataIndex: 'keyword', hideInTable: true },
    { title: '昵称', dataIndex: 'display_name', ellipsis: true, search: false },
    { title: '手机号', dataIndex: 'phone', width: 150, search: false },
    {
      title: '角色',
      dataIndex: 'role',
      width: 100,
      render: (_, row) => userRoleTag(row),
      valueEnum: {
        admin: { text: '管理员' },
        user: { text: '用户' },
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) => userStatusTag(row),
      valueEnum: {
        '-1': { text: '全部' },
        '1': { text: '启用' },
        '2': { text: '禁用' },
      },
    },
    { title: '创建时间', dataIndex: 'create_time', width: 180, search: false },
    {
      title: '操作',
      width: 230,
      search: false,
      render: (_, row) => {
        const disablingSelf = row.user_id === currentUserId && row.status !== 2;
        return (
          <Space size={4}>
            <Button type="link" size="small" onClick={() => showDetail(row.user_id)}>
              详情
            </Button>
            <Button
              type="link"
              size="small"
              onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/orders', userId: row.user_id } }))}
            >
              订单
            </Button>
            <Button
              type="link"
              size="small"
              onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/security', userId: row.user_id } }))}
            >
              日志
            </Button>
            <Popconfirm
              title={row.status === 2 ? '确认启用该用户？' : '确认禁用该用户？'}
              disabled={disablingSelf}
              onConfirm={() => updateStatus(row)}
            >
              <Button
                type="link"
                size="small"
                danger={row.status !== 2}
                disabled={disablingSelf}
              >
                {row.status === 2 ? '启用' : '禁用'}
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <>
      <ProTable<AdminUserItem>
        actionRef={actionRef}
        columns={columns}
        rowKey="user_id"
        params={{ initialUserId }}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const query = new URLSearchParams();
          query.set('page', String(params.current || 1));
          query.set('page_size', String(params.pageSize || 20));
          const keyword = params.keyword || params.initialUserId;
          if (keyword) query.set('keyword', String(keyword));
          if (params.role) query.set('role', String(params.role));
          if (params.status !== undefined && String(params.status) !== '-1') query.set('status', String(params.status));

          const res = await authed<AdminUserListResp>(`/api/admin/users?${query}`);
          return {
            data: res.ok ? res.data.items || [] : [],
            total: res.ok ? res.data.total : 0,
            success: res.ok,
          };
        }}
        pagination={{ defaultPageSize: 20 }}
      />
      <Modal
        title="用户详情"
        open={detailOpen}
        footer={[
          <Button key="close" onClick={() => setDetailOpen(false)}>关闭</Button>,
          <Button
            key="status"
            danger={detail?.status !== 2}
            disabled={!detail || (detail.status !== 2 && detail.user_id === currentUserId)}
            onClick={() => {
              if (!detail) return;
              void updateStatus(detail);
            }}
          >
            {detail?.status === 2 ? '启用' : '禁用'}
          </Button>,
          <Button
            key="orders"
            type="primary"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/orders', userId: detail.user_id } }));
            }}
          >
            查看订单
          </Button>,
          <Button
            key="security"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/security', userId: detail.user_id } }));
            }}
          >
            安全日志
          </Button>,
        ]}
        destroyOnHidden
        onCancel={() => setDetailOpen(false)}
      >
        {detail ? (
          <Descriptions column={2} bordered size="small">
            <Descriptions.Item label="用户ID">{detail.user_id}</Descriptions.Item>
            <Descriptions.Item label="状态">{userStatusTag(detail)}</Descriptions.Item>
            <Descriptions.Item label="昵称">{detail.display_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="角色">{userRoleTag(detail)}</Descriptions.Item>
            <Descriptions.Item label="手机号">{detail.phone || '-'}</Descriptions.Item>
            <Descriptions.Item label="创建时间">{detail.create_time || '-'}</Descriptions.Item>
          </Descriptions>
        ) : detailLoading ? (
          <div>加载中...</div>
        ) : (
          <div>暂无用户详情</div>
        )}
      </Modal>
    </>
  );
}
