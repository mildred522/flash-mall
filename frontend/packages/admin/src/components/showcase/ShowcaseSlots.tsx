import { useState } from 'react';
import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined } from '@ant-design/icons';
import { Button, Card, Space, Tag, Tooltip } from 'antd';
import { formatPriceFen, resolveProductImage } from '@flash-mall/shared';
import type { DraftSlot } from './showcaseModel';
import { invalidShowcaseReasonText } from './showcaseModel';
import ProductThumbnail from '../products/ProductThumbnail';

type Props = {
  slots: DraftSlot[];
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
};

export default function ShowcaseSlots({ slots, onMove, onRemove }: Props) {
  const [dragFrom, setDragFrom] = useState<number | null>(null);
  return (
    <Card title="12 个首页槽位"
      extra={<span style={{ color: '#8c8c8c' }}>{slots.filter((slot) => slot.product_id > 0).length} / 12 已使用</span>}>
      <div style={{ display: 'grid', gap: 10 }}>
        {slots.map((slot, index) => {
          const imageURL = slot.product ? resolveProductImage(slot.product.product_id, slot.product.image_url) : '';
          return (
            <div key={slot.slot_no} data-testid={`showcase-slot-${slot.slot_no}`} draggable={slot.product_id > 0}
              onDragStart={() => setDragFrom(index)} onDragOver={(event) => event.preventDefault()}
              onDrop={() => { if (dragFrom !== null) onMove(dragFrom, index); setDragFrom(null); }}
              style={{
                display: 'grid', gridTemplateColumns: '44px 62px minmax(0,1fr) auto', alignItems: 'center',
                gap: 12, minHeight: 76, padding: 10,
                border: `1px ${slot.product_id > 0 ? 'solid' : 'dashed'} ${slot.valid ? '#e5e5e5' : '#ff9c6e'}`,
                borderRadius: 10, background: slot.valid ? '#fff' : '#fff7e6',
              }}>
              <strong style={{ color: '#b23a2c', textAlign: 'center' }}>{String(slot.slot_no).padStart(2, '0')}</strong>
              <div style={{ width: 62, height: 54, display: 'grid', placeItems: 'center', overflow: 'hidden', borderRadius: 8, background: '#f7efe8', color: '#b23a2c' }}>
                {imageURL ? <ProductThumbnail src={imageURL} alt={`${slot.product?.name || '商品'} 商品图`} width={62} height={54} /> : slot.product_id > 0 ? '商品' : '＋'}
              </div>
              {slot.product_id > 0 ? (
                <div style={{ minWidth: 0 }}>
                  <strong style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {slot.product?.name || `商品 ${slot.product_id}`}
                  </strong>
                  <Space size={8} wrap style={{ marginTop: 5, fontSize: 12, color: '#8c8c8c' }}>
                    <span>{slot.product?.merchant_name || `商品 ID ${slot.product_id}`}</span>
                    {slot.product && <span>¥{formatPriceFen(slot.product.final_price_fen)}</span>}
                    {!slot.valid && <Tag color="error">{invalidShowcaseReasonText[slot.invalid_reason || ''] || '当前不可展示'}</Tag>}
                  </Space>
                </div>
              ) : <div><strong>空槽位</strong><div style={{ color: '#aaa', fontSize: 12, marginTop: 4 }}>可从右侧候选加入</div></div>}
              <Space size={2}>
                <Tooltip title="上移"><Button aria-label="上移" size="small" icon={<ArrowUpOutlined />}
                  disabled={index === 0} onClick={() => onMove(index, index - 1)} /></Tooltip>
                <Tooltip title="下移"><Button aria-label="下移" size="small" icon={<ArrowDownOutlined />}
                  disabled={index === slots.length - 1} onClick={() => onMove(index, index + 1)} /></Tooltip>
                <Tooltip title="移除"><Button aria-label="移除" size="small" danger icon={<DeleteOutlined />}
                  disabled={slot.product_id <= 0} onClick={() => onRemove(index)} /></Tooltip>
              </Space>
            </div>
          );
        })}
      </div>
    </Card>
  );
}
