import { Button, Card, Form, Input, Upload } from 'antd';
import type { FormInstance } from 'antd';
import { SaveOutlined, UploadOutlined } from '@ant-design/icons';
import type { MerchantStoreProfile } from '@flash-mall/shared';
import type { StoreAssetType, StoreFormValues } from './storeModel';

type Props = {
  form: FormInstance<StoreFormValues>;
  profile: MerchantStoreProfile | null;
  descriptionLength: number;
  saving: boolean;
  uploading: StoreAssetType | null;
  onUpload: (file: File, assetType: StoreAssetType) => Promise<boolean | string>;
  onSave: () => void;
};

export default function StoreProfileForm(props: Props) {
  return (
    <Card
      title="店铺资料"
      extra={<span style={{ color: '#8c8c8c', fontSize: 12 }}>当前版本 {props.profile?.version ?? 0}</span>}
      style={{ height: '100%' }}
    >
      <Form form={props.form} layout="vertical" requiredMark={false} onFinish={props.onSave}>
        <Form.Item name="logo_url" hidden><Input /></Form.Item>
        <Form.Item name="banner_url" hidden><Input /></Form.Item>

        <Form.Item label="店铺 Logo" extra="建议使用方形图片，公开页面将裁剪为圆形。">
          <div data-testid="logo-upload">
            <Upload
              accept="image/jpeg,image/png,image/webp"
              maxCount={1}
              showUploadList={false}
              beforeUpload={(file) => props.onUpload(file, 'logo')}
            >
              <Button icon={<UploadOutlined />} loading={props.uploading === 'logo'}>上传 Logo</Button>
            </Upload>
          </div>
        </Form.Item>

        <Form.Item label="店铺横幅" extra="建议使用 16:6 横向图片，文字应避开边缘。">
          <div data-testid="banner-upload">
            <Upload
              accept="image/jpeg,image/png,image/webp"
              maxCount={1}
              showUploadList={false}
              beforeUpload={(file) => props.onUpload(file, 'banner')}
            >
              <Button icon={<UploadOutlined />} loading={props.uploading === 'banner'}>上传横幅</Button>
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
          {props.descriptionLength} / 300
        </div>

        <Button
          type="primary"
          htmlType="submit"
          icon={<SaveOutlined />}
          loading={props.saving}
          disabled={!props.profile || Boolean(props.uploading)}
          block
          size="large"
        >
          保存店铺资料
        </Button>
      </Form>
    </Card>
  );
}
