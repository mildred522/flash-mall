import { useEffect, useState } from 'react';
import { Alert, Button, Card, Col, Form, Input, Row, Space, Spin, Upload, message } from 'antd';
import { EyeOutlined, SaveOutlined, UploadOutlined } from '@ant-design/icons';
import { authed, uploadImageAsset } from '@flash-mall/shared';
import type { MerchantStoreProfile } from '@flash-mall/shared';

type StoreForm = Pick<MerchantStoreProfile, 'logo_url' | 'banner_url' | 'description'>;
type AssetType = 'logo' | 'banner';

export default function StoreSettingsPage() {
  const [form] = Form.useForm<StoreForm>();
  const [profile, setProfile] = useState<MerchantStoreProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState<AssetType | null>(null);
  const [notice, setNotice] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const logoURL = Form.useWatch('logo_url', form) || '';
  const bannerURL = Form.useWatch('banner_url', form) || '';
  const description = Form.useWatch('description', form) || '';

  useEffect(() => {
    authed<MerchantStoreProfile>('/api/merchant/store/profile')
      .then((response) => {
        if (!response.ok) {
          setNotice({ type: 'error', text: '店铺资料加载失败，请稍后重试' });
          return;
        }
        setProfile(response.data);
        form.setFieldsValue({
          logo_url: response.data.logo_url || '',
          banner_url: response.data.banner_url || '',
          description: response.data.description || '',
        });
      })
      .catch(() => setNotice({ type: 'error', text: '店铺资料加载失败，请稍后重试' }))
      .finally(() => setLoading(false));
  }, [form]);

  const uploadAsset = async (file: File, assetType: AssetType) => {
    setUploading(assetType);
    setNotice(null);
    try {
      const imageURL = await uploadImageAsset(file, {
        endpoint: '/api/merchant/store/assets',
        fields: { asset_type: assetType },
      });
      form.setFieldValue(assetType === 'logo' ? 'logo_url' : 'banner_url', imageURL);
      message.success(assetType === 'logo' ? 'Logo 上传完成' : '横幅上传完成');
    } catch (error) {
      setNotice({ type: 'error', text: error instanceof Error ? error.message : '图片上传失败' });
    } finally {
      setUploading(null);
    }
    return Upload.LIST_IGNORE;
  };

  const save = async () => {
    if (!profile) return;
    const values = await form.validateFields();
    setSaving(true);
    setNotice(null);
    try {
      const response = await authed<MerchantStoreProfile>('/api/merchant/store/profile', {
        method: 'POST',
        jsonBody: {
          logo_url: values.logo_url?.trim() || '',
          banner_url: values.banner_url?.trim() || '',
          description: values.description?.trim() || '',
          expected_version: profile.version,
        },
      });
      if (response.status === 409) {
        setNotice({ type: 'error', text: '资料已被更新，请刷新后重试' });
        return;
      }
      if (!response.ok) {
        setNotice({ type: 'error', text: '店铺资料保存失败，请稍后重试' });
        return;
      }
      setProfile(response.data);
      form.setFieldsValue({
        logo_url: response.data.logo_url || '',
        banner_url: response.data.banner_url || '',
        description: response.data.description || '',
      });
      setNotice({ type: 'success', text: '店铺资料已保存' });
    } catch {
      setNotice({ type: 'error', text: '店铺资料保存失败，请稍后重试' });
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div style={{ minHeight: 360, display: 'grid', placeItems: 'center' }}><Spin size="large" /></div>;

  return (
    <div style={{ maxWidth: 1180, margin: '0 auto' }}>
      <Space direction="vertical" size={4} style={{ marginBottom: 20 }}>
        <h1 style={{ margin: 0 }}>店铺设置</h1>
        <span style={{ color: '#7b7b7b' }}>用清晰的品牌资料建立自己的店面，保存后公开店铺会立即使用最新版本。</span>
      </Space>

      {notice && <Alert style={{ marginBottom: 18 }} showIcon type={notice.type} message={notice.text} />}

      <Row gutter={[20, 20]} align="stretch">
        <Col xs={24} lg={10}>
          <Card
            title="店铺资料"
            extra={<span style={{ color: '#8c8c8c', fontSize: 12 }}>当前版本 {profile?.version ?? 0}</span>}
            style={{ height: '100%' }}
          >
            <Form form={form} layout="vertical" requiredMark={false} onFinish={save}>
              <Form.Item name="logo_url" hidden><Input /></Form.Item>
              <Form.Item name="banner_url" hidden><Input /></Form.Item>

              <Form.Item label="店铺 Logo" extra="建议使用方形图片，公开页面将裁剪为圆形。">
                <div data-testid="logo-upload">
                  <Upload
                    accept="image/jpeg,image/png,image/webp"
                    maxCount={1}
                    showUploadList={false}
                    beforeUpload={(file) => uploadAsset(file, 'logo')}
                  >
                    <Button icon={<UploadOutlined />} loading={uploading === 'logo'}>上传 Logo</Button>
                  </Upload>
                </div>
              </Form.Item>

              <Form.Item label="店铺横幅" extra="建议使用 16:6 横向图片，文字应避开边缘。">
                <div data-testid="banner-upload">
                  <Upload
                    accept="image/jpeg,image/png,image/webp"
                    maxCount={1}
                    showUploadList={false}
                    beforeUpload={(file) => uploadAsset(file, 'banner')}
                  >
                    <Button icon={<UploadOutlined />} loading={uploading === 'banner'}>上传横幅</Button>
                  </Upload>
                </div>
              </Form.Item>

              <Form.Item
                name="description"
                label="店铺简介"
                rules={[{ max: 300, message: '店铺简介不能超过 300 字' }]}
              >
                <Input.TextArea rows={6} maxLength={300} placeholder="介绍你的选品方向、服务特色和品牌故事" />
              </Form.Item>
              <div data-testid="description-count" style={{ marginTop: -18, marginBottom: 20, color: '#8c8c8c', textAlign: 'right' }}>
                {description.length} / 300
              </div>

              <Button
                type="primary"
                htmlType="submit"
                icon={<SaveOutlined />}
                loading={saving}
                disabled={!profile || Boolean(uploading)}
                block
                size="large"
              >
                保存店铺资料
              </Button>
            </Form>
          </Card>
        </Col>

        <Col xs={24} lg={14}>
          <Card title={<Space><EyeOutlined />公开店铺预览</Space>} style={{ height: '100%' }} styles={{ body: { padding: 0 } }}>
            <div
              data-testid="store-preview-banner"
              style={{
                minHeight: 330,
                padding: 40,
                display: 'flex',
                alignItems: 'flex-end',
                gap: 24,
                color: bannerURL ? '#fff' : '#31241c',
                backgroundImage: bannerURL
                  ? `linear-gradient(90deg, rgba(28,22,18,.86), rgba(28,22,18,.22)), url(${bannerURL})`
                  : 'linear-gradient(125deg, #fff2e4, #e9cdb3)',
                backgroundPosition: 'center',
                backgroundSize: 'cover',
              }}
            >
              <div style={{
                width: 96,
                height: 96,
                flex: '0 0 auto',
                display: 'grid',
                placeItems: 'center',
                overflow: 'hidden',
                borderRadius: '50%',
                border: '4px solid rgba(255,255,255,.82)',
                background: '#b23a2c',
                color: '#fff',
                fontSize: 32,
                fontWeight: 800,
                boxShadow: '0 12px 28px rgba(0,0,0,.18)',
              }}>
                {logoURL ? <img src={logoURL} alt="店铺 Logo 预览" style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : profile?.merchant_name?.slice(0, 1) || '店'}
              </div>
              <div>
                <span style={{ fontSize: 11, fontWeight: 800, letterSpacing: 2, opacity: .8 }}>MERCHANT STOREFRONT</span>
                <h2 style={{ margin: '7px 0', fontSize: 42 }}>{profile?.merchant_name || '我的店铺'}</h2>
                <p style={{ maxWidth: 560, margin: 0, lineHeight: 1.7 }}>
                  {description || '写一段简洁的店铺介绍，让顾客知道你在认真挑选什么。'}
                </p>
              </div>
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
