import { Button, Modal, Table } from 'antd';
import type { AdminOrderStatusLogItem } from '@flash-mall/shared';

type Props = {
  open: boolean;
  loading: boolean;
  orderId: string;
  logs: AdminOrderStatusLogItem[];
  onClose: () => void;
  onSecurity: (orderId: string) => void;
  onUser: (userId: number) => void;
};

export default function OrderStatusLogsModal(props: Props) {
  const { open, loading, orderId, logs, onClose, onSecurity, onUser } = props;
  return (
    <Modal title={`状态日志 ${orderId}`} open={open} onCancel={onClose} width={820} footer={[
      <Button key="close" onClick={onClose}>关闭</Button>,
      <Button key="security" type="primary" disabled={!orderId} onClick={() => {
        onClose();
        onSecurity(orderId);
      }}>安全日志</Button>,
    ]}>
      <Table<AdminOrderStatusLogItem> rowKey="id" loading={loading} dataSource={logs} pagination={false} size="small" columns={[
        { title: '时间', dataIndex: 'create_time', width: 170 },
        { title: '原状态', dataIndex: 'from_status_text', width: 100 },
        { title: '新状态', dataIndex: 'to_status_text', width: 100 },
        { title: '操作人', dataIndex: 'operator_id', width: 100, render: (_, row) => row.operator_id
          ? <Button type="link" size="small" onClick={() => onUser(row.operator_id)}>{row.operator_id}</Button>
          : '-' },
        { title: '备注', dataIndex: 'remark', ellipsis: true },
      ]} />
    </Modal>
  );
}
