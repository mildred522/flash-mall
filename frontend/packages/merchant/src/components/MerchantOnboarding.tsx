import { useEffect, useState } from 'react';
import { Alert, Button, Card, Descriptions, Form, Input, Result, Space, Typography, message } from 'antd';
import { authed } from '@flash-mall/shared';
import type { MeResp, MerchantApplicationItem, MerchantApplyResp } from '@flash-mall/shared';

const { Paragraph, Title } = Typography;

interface MerchantOnboardingProps {
  application: MerchantApplicationItem | null;
  disabled: boolean;
  initializing: boolean;
  onRefresh: () => void;
  onSwitchAccount: () => void;
}

type ApplyValues = {
  merchant_name: string;
  contact_phone: string;
};

export default function MerchantOnboarding({
  application,
  disabled,
  initializing,
  onRefresh,
  onSwitchAccount,
}: MerchantOnboardingProps) {
  if (disabled) {
    return (
      <Result
        status="error"
        title="商家已停用"
        subTitle="当前账号绑定的商家已被平台停用，请联系平台管理员处理。"
        extra={<Button onClick={onSwitchAccount}>切换账号</Button>}
      />
    );
  }

  if (initializing) {
    return (
      <Result
        status="info"
        title="正在初始化商家身份"
        subTitle="申请已经通过，商家绑定正在生效。"
        extra={(
          <Space>
            <Button type="primary" onClick={onRefresh}>刷新商家身份</Button>
            <Button onClick={onSwitchAccount}>切换账号</Button>
          </Space>
        )}
      />
    );
  }

  if (application?.status === 0) {
    return (
      <Result
        status="info"
        title="申请审核中"
        subTitle={`申请编号：${application.apply_id}`}
        extra={(
          <Space direction="vertical" size="middle">
            <Descriptions bordered size="small" column={1}>
              <Descriptions.Item label="商家名称">{application.merchant_name}</Descriptions.Item>
              <Descriptions.Item label="联系电话">{application.contact_phone || '-'}</Descriptions.Item>
              <Descriptions.Item label="申请时间">{application.create_time || '-'}</Descriptions.Item>
            </Descriptions>
            <Space>
              <Button type="primary" onClick={onRefresh}>刷新审核状态</Button>
              <Button onClick={onSwitchAccount}>切换账号</Button>
            </Space>
          </Space>
        )}
      />
    );
  }

  return (
    <ApplicationForm
      application={application?.status === 2 ? application : null}
      onRefresh={onRefresh}
      onSwitchAccount={onSwitchAccount}
    />
  );
}

function ApplicationForm({
  application,
  onRefresh,
  onSwitchAccount,
}: {
  application: MerchantApplicationItem | null;
  onRefresh: () => void;
  onSwitchAccount: () => void;
}) {
  const [form] = Form.useForm<ApplyValues>();
  const [submitting, setSubmitting] = useState(false);
  const rejected = application?.status === 2;

  useEffect(() => {
    if (application) {
      form.setFieldsValue({
        merchant_name: application.merchant_name,
        contact_phone: application.contact_phone,
      });
      return;
    }
    form.resetFields();
    void authed<MeResp>('/api/auth/me').then((response) => {
      if (response.ok && response.data.phone) {
        form.setFieldValue('contact_phone', response.data.phone);
      }
    });
  }, [application, form]);

  const submit = async (values: ApplyValues) => {
    setSubmitting(true);
    try {
      const response = await authed<MerchantApplyResp>('/api/merchant/apply', {
        method: 'POST',
        jsonBody: {
          merchant_name: values.merchant_name.trim(),
          contact_phone: values.contact_phone.trim(),
        },
      });
      if (!response.ok) {
        message.error(response.status === 409 ? '当前账号已经绑定商家' : '申请提交失败，请稍后重试');
        return;
      }
      message.success('入驻申请已提交');
      onRefresh();
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={{ display: 'flex', minHeight: '100vh', alignItems: 'center', justifyContent: 'center', padding: 24, background: '#f5f7fa' }}>
      <Card style={{ width: 560 }}>
        <Title level={3}>{rejected ? '修改后重新申请' : '申请成为商家'}</Title>
        <Paragraph type="secondary">审核通过后，当前普通账号将获得店主身份并进入经营后台。</Paragraph>
        {rejected ? (
          <Alert
            type="error"
            showIcon
            message="上次申请未通过"
            description={application.audit_remark || '平台未填写驳回原因'}
            style={{ marginBottom: 20 }}
          />
        ) : null}
        <Form<ApplyValues> form={form} layout="vertical" onFinish={submit} autoComplete="off">
          <Form.Item
            label="商家名称"
            name="merchant_name"
            rules={[{ required: true, whitespace: true, message: '请输入商家名称' }]}
          >
            <Input maxLength={128} placeholder="例如：Flash Mall 数码店" />
          </Form.Item>
          <Form.Item
            label="联系电话"
            name="contact_phone"
            rules={[
              { required: true, message: '请输入联系电话' },
              { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号' },
            ]}
          >
            <Input maxLength={11} placeholder="用于平台联系店主" />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={submitting}>
              {rejected ? '重新提交申请' : '提交申请'}
            </Button>
            <Button onClick={onSwitchAccount}>切换账号</Button>
          </Space>
        </Form>
      </Card>
    </div>
  );
}
