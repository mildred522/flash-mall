import { Card, Space } from 'antd';
import { EyeOutlined } from '@ant-design/icons';

type Props = {
  merchantName: string;
  logoURL: string;
  bannerURL: string;
  description: string;
};

export default function StorePreview(props: Props) {
  return (
    <Card title={<Space><EyeOutlined />公开店铺预览</Space>} style={{ height: '100%' }} styles={{ body: { padding: 0 } }}>
      <div
        data-testid="store-preview-banner"
        style={{
          minHeight: 330,
          padding: 40,
          display: 'flex',
          alignItems: 'flex-end',
          gap: 24,
          color: props.bannerURL ? '#fff' : '#31241c',
          backgroundImage: props.bannerURL
            ? `linear-gradient(90deg, rgba(28,22,18,.86), rgba(28,22,18,.22)), url(${props.bannerURL})`
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
          {props.logoURL
            ? <img src={props.logoURL} alt="店铺 Logo 预览" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
            : props.merchantName.slice(0, 1) || '店'}
        </div>
        <div>
          <span style={{ fontSize: 11, fontWeight: 800, letterSpacing: 2, opacity: .8 }}>MERCHANT STOREFRONT</span>
          <h2 style={{ margin: '7px 0', fontSize: 42 }}>{props.merchantName || '我的店铺'}</h2>
          <p style={{ maxWidth: 560, margin: 0, lineHeight: 1.7 }}>
            {props.description || '写一段简洁的店铺介绍，让顾客知道你在认真挑选什么。'}
          </p>
        </div>
      </div>
    </Card>
  );
}
