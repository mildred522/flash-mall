import { Tag } from 'antd';
import type { AdminMutationResp, AdminUserItem } from '@flash-mall/shared';

export function userMutationError(data: AdminMutationResp): string {
  if (data.error === 'cannot disable current admin') return '不能禁用当前登录的管理员账号';
  return data.error || '';
}

export function UserStatusTag({ user }: { user: Pick<AdminUserItem, 'status'> }) {
  return user.status === 2 ? <Tag color="red">禁用</Tag> : <Tag color="green">启用</Tag>;
}

export function UserRoleTag({ user }: { user: Pick<AdminUserItem, 'role'> }) {
  return user.role === 'admin' ? <Tag color="red">管理员</Tag> : <Tag color="blue">用户</Tag>;
}

export function navigateFromUser(path: string, userId: number) {
  window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path, userId } }));
}
