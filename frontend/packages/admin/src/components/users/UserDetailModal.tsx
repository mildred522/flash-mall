import { Button, Descriptions, Modal } from 'antd';
import type { AdminUserItem } from '@flash-mall/shared';
import { navigateFromUser, UserRoleTag, UserStatusTag } from './userModel';

type Props = {
  open: boolean;
  loading: boolean;
  detail: AdminUserItem | null;
  currentUserId: number;
  onClose: () => void;
  onStatus: (user: AdminUserItem) => void;
};

export default function UserDetailModal({ open, loading, detail, currentUserId, onClose, onStatus }: Props) {
  const navigate = (path: string) => {
    if (!detail) return;
    onClose();
    navigateFromUser(path, detail.user_id);
  };
  return (
    <Modal title="用户详情" open={open} destroyOnHidden onCancel={onClose} footer={[
      <Button key="close" onClick={onClose}>关闭</Button>,
      <Button key="status" danger={detail?.status !== 2}
        disabled={!detail || (detail.status !== 2 && detail.user_id === currentUserId)}
        onClick={() => detail && onStatus(detail)}>
        {detail?.status === 2 ? '启用' : '禁用'}
      </Button>,
      <Button key="orders" type="primary" disabled={!detail} onClick={() => navigate('/admin/orders')}>查看订单</Button>,
      <Button key="security" disabled={!detail} onClick={() => navigate('/admin/security')}>安全日志</Button>,
    ]}>
      {detail ? (
        <Descriptions column={2} bordered size="small">
          <Descriptions.Item label="用户ID">{detail.user_id}</Descriptions.Item>
          <Descriptions.Item label="状态"><UserStatusTag user={detail} /></Descriptions.Item>
          <Descriptions.Item label="昵称">{detail.display_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="角色"><UserRoleTag user={detail} /></Descriptions.Item>
          <Descriptions.Item label="手机号">{detail.phone || '-'}</Descriptions.Item>
          <Descriptions.Item label="创建时间">{detail.create_time || '-'}</Descriptions.Item>
        </Descriptions>
      ) : loading ? <div>加载中...</div> : <div>暂无用户详情</div>}
    </Modal>
  );
}
