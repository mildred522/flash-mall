import { Button, Popconfirm, Space } from 'antd';
import type { ProColumns } from '@ant-design/pro-components';
import type { AdminUserItem } from '@flash-mall/shared';
import { navigateFromUser, UserRoleTag, UserStatusTag } from './userModel';

type Actions = {
  detail: (userId: number) => void;
  status: (user: AdminUserItem) => void;
};

export function createUserColumns(currentUserId: number, actions: Actions): ProColumns<AdminUserItem>[] {
  return [
    { title: '用户ID', dataIndex: 'user_id', width: 100, search: false },
    { title: '关键词', dataIndex: 'keyword', hideInTable: true },
    { title: '昵称', dataIndex: 'display_name', ellipsis: true, search: false },
    { title: '手机号', dataIndex: 'phone', width: 150, search: false },
    { title: '角色', dataIndex: 'role', width: 100, render: (_, row) => <UserRoleTag user={row} />,
      valueEnum: { admin: { text: '管理员' }, user: { text: '用户' } } },
    { title: '状态', dataIndex: 'status', width: 100, render: (_, row) => <UserStatusTag user={row} />,
      valueEnum: { '-1': { text: '全部' }, '1': { text: '启用' }, '2': { text: '禁用' } } },
    { title: '创建时间', dataIndex: 'create_time', width: 180, search: false },
    { title: '操作', width: 230, search: false, render: (_, row) => {
      const disablingSelf = row.user_id === currentUserId && row.status !== 2;
      return (
        <Space size={4}>
          <Button type="link" size="small" onClick={() => actions.detail(row.user_id)}>详情</Button>
          <Button type="link" size="small" onClick={() => navigateFromUser('/admin/orders', row.user_id)}>订单</Button>
          <Button type="link" size="small" onClick={() => navigateFromUser('/admin/security', row.user_id)}>日志</Button>
          <Popconfirm title={row.status === 2 ? '确认启用该用户？' : '确认禁用该用户？'}
            disabled={disablingSelf} onConfirm={() => actions.status(row)}>
            <Button type="link" size="small" danger={row.status !== 2} disabled={disablingSelf}>
              {row.status === 2 ? '启用' : '禁用'}
            </Button>
          </Popconfirm>
        </Space>
      );
    } },
  ];
}
