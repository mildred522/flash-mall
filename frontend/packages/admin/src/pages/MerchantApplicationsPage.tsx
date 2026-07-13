import { useRef, useState } from 'react';
import { Button, Form, Input, Modal, Space, Tag, message } from 'antd';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed } from '@flash-mall/shared';
import type {
  AdminMerchantApplicationItem,
  AdminMerchantApplicationListResp,
  AdminMerchantAuditResp,
} from '@flash-mall/shared';

type AuditMode = 'approve' | 'reject';

type AuditFormValues = {
  remark: string;
};

function statusTag(status: number) {
  if (status === 1) return <Tag color="green">已通过</Tag>;
  if (status === 2) return <Tag color="red">已驳回</Tag>;
  return <Tag color="gold">待审核</Tag>;
}

export default function MerchantApplicationsPage() {
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [form] = Form.useForm<AuditFormValues>();
  const [auditing, setAuditing] = useState<AdminMerchantApplicationItem | null>(null);
  const [auditMode, setAuditMode] = useState<AuditMode>('approve');
  const [submitting, setSubmitting] = useState(false);

  const openAudit = (application: AdminMerchantApplicationItem, mode: AuditMode) => {
    setAuditing(application);
    setAuditMode(mode);
    form.setFieldsValue({ remark: '' });
  };

  const closeAudit = () => {
    setAuditing(null);
    form.resetFields();
  };

  const submitAudit = async () => {
    if (!auditing) return;
    let values: AuditFormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    const approve = auditMode === 'approve';
    setSubmitting(true);
    try {
      const response = await authed<AdminMerchantAuditResp>('/api/admin/merchants/applications/audit', {
        method: 'POST',
        jsonBody: {
          apply_id: auditing.apply_id,
          approve,
          remark: values.remark?.trim() || '',
        },
      });
      if (!response.ok) {
        message.error(response.status === 409 ? '该申请已经处理' : '审核提交失败');
        return;
      }
      message.success(approve ? '入驻申请已通过' : '入驻申请已驳回');
      closeAudit();
      actionRef.current?.reload();
    } finally {
      setSubmitting(false);
    }
  };

  const columns: ProColumns<AdminMerchantApplicationItem>[] = [
    { title: '申请ID', dataIndex: 'apply_id', width: 100, search: false },
    { title: '用户ID', dataIndex: 'user_id', width: 100, search: false },
    { title: '商家名称', dataIndex: 'merchant_name', ellipsis: true, search: false },
    { title: '联系电话', dataIndex: 'contact_phone', width: 150, search: false },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      valueEnum: {
        '-1': { text: '全部' },
        '0': { text: '待审核' },
        '1': { text: '已通过' },
        '2': { text: '已驳回' },
      },
      render: (_, row) => statusTag(row.status),
    },
    { title: '申请时间', dataIndex: 'create_time', width: 180, search: false },
    { title: '审核意见', dataIndex: 'audit_remark', ellipsis: true, search: false },
    { title: '操作人', dataIndex: 'operator_id', width: 100, search: false, renderText: (value) => value || '-' },
    { title: '审核时间', dataIndex: 'audit_time', width: 180, search: false, renderText: (value) => value || '-' },
    {
      title: '操作',
      width: 140,
      search: false,
      fixed: 'right',
      render: (_, row) => row.status === 0 ? (
        <Space size={4}>
          <Button type="link" size="small" onClick={() => openAudit(row, 'approve')}>通过</Button>
          <Button type="link" size="small" danger onClick={() => openAudit(row, 'reject')}>驳回</Button>
        </Space>
      ) : '-',
    },
  ];

  return (
    <>
      <ProTable<AdminMerchantApplicationItem>
        headerTitle="商家入驻申请"
        actionRef={actionRef}
        columns={columns}
        rowKey="apply_id"
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const query = new URLSearchParams();
          query.set('page', String(params.current || 1));
          query.set('page_size', String(params.pageSize || 20));
          if (params.status !== undefined && String(params.status) !== '-1') {
            query.set('status', String(params.status));
          }
          const response = await authed<AdminMerchantApplicationListResp>(
            `/api/admin/merchants/applications?${query}`,
          );
          return {
            data: response.ok ? response.data.items || [] : [],
            total: response.ok ? response.data.total : 0,
            success: response.ok,
          };
        }}
        pagination={{ defaultPageSize: 20 }}
        scroll={{ x: 1400 }}
      />
      <Modal
        title={auditMode === 'approve' ? '通过入驻申请' : '驳回入驻申请'}
        open={Boolean(auditing)}
        okText={auditMode === 'approve' ? '确认通过' : '确认驳回'}
        confirmLoading={submitting}
        destroyOnHidden
        onOk={() => void submitAudit()}
        onCancel={closeAudit}
      >
        <p>
          {auditing ? `${auditing.merchant_name}（申请 ID：${auditing.apply_id}）` : ''}
        </p>
        <Form<AuditFormValues> form={form} layout="vertical">
          <Form.Item
            label={auditMode === 'approve' ? '审核备注' : '驳回原因'}
            name="remark"
            rules={auditMode === 'reject' ? [{ required: true, whitespace: true, message: '请输入驳回原因' }] : []}
          >
            <Input.TextArea rows={4} maxLength={500} placeholder={auditMode === 'reject' ? '说明需要补充或修正的材料' : '可选'} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
