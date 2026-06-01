import { useRef } from 'react';
import { Tag } from 'antd';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed } from '@flash-mall/shared';
import type { AdminUserItem, AdminUserListResp } from '@flash-mall/shared';

export default function UsersPage() {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns<AdminUserItem>[] = [
    { title: '用户ID', dataIndex: 'user_id', width: 100 },
    { title: '昵称', dataIndex: 'display_name', ellipsis: true },
    { title: '手机号', dataIndex: 'phone', width: 150 },
    {
      title: '角色', dataIndex: 'role', width: 100,
      render: (_, row) => row.role === 'admin' ? <Tag color="red">管理员</Tag> : <Tag color="blue">用户</Tag>,
    },
  ];

  return (
    <ProTable<AdminUserItem>
      actionRef={actionRef}
      columns={columns}
      rowKey="user_id"
      search={false}
      request={async (params) => {
        const query = new URLSearchParams();
        query.set('page', String(params.current || 1));
        query.set('page_size', String(params.pageSize || 20));

        const res = await authed<AdminUserListResp>(`/api/admin/users?${query}`);
        return {
          data: res.ok ? res.data.items || [] : [],
          total: res.ok ? res.data.total : 0,
          success: res.ok,
        };
      }}
      pagination={{ defaultPageSize: 20 }}
    />
  );
}
