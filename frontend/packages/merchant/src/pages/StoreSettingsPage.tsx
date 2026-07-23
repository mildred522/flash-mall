import { useEffect, useState } from 'react';
import { Alert, Col, Form, Row, Space, Spin, Upload, message } from 'antd';
import { authed, uploadImageAsset } from '@flash-mall/shared';
import type { MerchantStoreProfile } from '@flash-mall/shared';
import StoreProfileForm from '../components/store/StoreProfileForm';
import StorePreview from '../components/store/StorePreview';
import type { StoreAssetType, StoreFormValues } from '../components/store/storeModel';

type Notice = { type: 'success' | 'error'; text: string };

export default function StoreSettingsPage() {
  const [form] = Form.useForm<StoreFormValues>();
  const [profile, setProfile] = useState<MerchantStoreProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState<StoreAssetType | null>(null);
  const [notice, setNotice] = useState<Notice | null>(null);
  const logoURL = Form.useWatch('logo_url', form) || '';
  const bannerURL = Form.useWatch('banner_url', form) || '';
  const description = Form.useWatch('description', form) || '';

  const applyProfile = (nextProfile: MerchantStoreProfile) => {
    setProfile(nextProfile);
    form.setFieldsValue({
      logo_url: nextProfile.logo_url || '',
      banner_url: nextProfile.banner_url || '',
      description: nextProfile.description || '',
    });
  };

  useEffect(() => {
    authed<MerchantStoreProfile>('/api/merchant/store/profile')
      .then((response) => {
        if (response.ok) applyProfile(response.data);
        else setNotice({ type: 'error', text: '店铺资料加载失败，请稍后重试' });
      })
      .catch(() => setNotice({ type: 'error', text: '店铺资料加载失败，请稍后重试' }))
      .finally(() => setLoading(false));
  }, [form]);

  const uploadAsset = async (file: File, assetType: StoreAssetType) => {
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
      } else if (!response.ok) {
        setNotice({ type: 'error', text: '店铺资料保存失败，请稍后重试' });
      } else {
        applyProfile(response.data);
        setNotice({ type: 'success', text: '店铺资料已保存' });
      }
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
          <StoreProfileForm
            form={form}
            profile={profile}
            descriptionLength={description.length}
            saving={saving}
            uploading={uploading}
            onUpload={uploadAsset}
            onSave={save}
          />
        </Col>
        <Col xs={24} lg={14}>
          <StorePreview
            merchantName={profile?.merchant_name || '我的店铺'}
            logoURL={logoURL}
            bannerURL={bannerURL}
            description={description}
          />
        </Col>
      </Row>
    </div>
  );
}
