import { PlusOutlined } from '@ant-design/icons';
import { Button, Card, Progress, Tag, Tooltip } from 'antd';
import type { ShowcaseCandidate } from '@flash-mall/shared';
import type { DraftSlot } from './showcaseModel';
import { candidateDisabledReason } from './showcaseModel';

type Props = { candidates: ShowcaseCandidate[]; slots: DraftSlot[]; onAdd: (candidate: ShowcaseCandidate) => void };

export default function ShowcaseCandidates({ candidates, slots, onAdd }: Props) {
  const merchantCounts = new Map<number, number>();
  slots.forEach((slot) => {
    const merchantId = slot.product?.merchant_id;
    if (merchantId) merchantCounts.set(merchantId, (merchantCounts.get(merchantId) || 0) + 1);
  });
  return (
    <Card title="推荐候选" extra={<Tag color="gold">仅建议，不会自动发布</Tag>}
      styles={{ body: { maxHeight: 'calc(100vh - 220px)', overflowY: 'auto' } }}>
      <div style={{ display: 'grid', gap: 14 }}>
        {candidates.map((candidate) => {
          const reason = candidateDisabledReason(candidate, slots);
          const merchantCount = candidate.product.merchant_id ? merchantCounts.get(candidate.product.merchant_id) || 0 : 0;
          return (
            <div key={candidate.product.product_id} style={{ border: '1px solid #eee', borderRadius: 12, padding: 14 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
                <div style={{ minWidth: 0 }}><strong>{candidate.product.name}</strong>
                  <div style={{ color: '#8c8c8c', fontSize: 12, marginTop: 4 }}>
                    {candidate.product.merchant_name || '未知商家'} · 已占 {merchantCount}/2
                  </div>
                </div>
                <div style={{ minWidth: 74, textAlign: 'right' }}>
                  <strong style={{ color: '#b23a2c', fontSize: 22 }}>{candidate.score}</strong><span style={{ color: '#aaa' }}> 分</span>
                </div>
              </div>
              <Progress percent={Math.min(100, Math.max(0, candidate.score))} showInfo={false} strokeColor="#b23a2c" size="small" />
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 4, margin: '10px 0', fontSize: 11, textAlign: 'center' }}>
                {[
                  ['销量', candidate.sales_score], ['库存', candidate.stock_score], ['促销', candidate.promotion_score],
                  ['新鲜', candidate.freshness_score], ['多样', candidate.diversity_score],
                ].map(([label, score]) => <span key={String(label)} style={{ padding: '5px 2px', borderRadius: 5, background: '#faf5f1' }}>{label}<br /><b>{score}</b></span>)}
              </div>
              <div style={{ color: '#6f6f6f', fontSize: 12, minHeight: 18 }}>{candidate.reasons.join(' · ')}</div>
              <Tooltip title={reason}>
                <Button block type="primary" ghost icon={<PlusOutlined />} disabled={Boolean(reason)}
                  aria-label={`加入${candidate.product.name}`} onClick={() => onAdd(candidate)} style={{ marginTop: 10 }}>
                  {reason || '加入第一个空槽'}
                </Button>
              </Tooltip>
            </div>
          );
        })}
      </div>
    </Card>
  );
}
