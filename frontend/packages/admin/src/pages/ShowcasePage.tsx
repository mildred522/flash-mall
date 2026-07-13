import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Col, Progress, Row, Space, Spin, Tag, Tooltip } from 'antd';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  PlusOutlined,
  ReloadOutlined,
  SendOutlined,
} from '@ant-design/icons';
import { authed, formatPriceFen, resolveProductImage } from '@flash-mall/shared';
import type {
  ShowcaseCandidate,
  ShowcaseCandidatesResp,
  ShowcasePublishReq,
  ShowcaseResp,
  ShowcaseSlot,
} from '@flash-mall/shared';

export type DraftSlot = ShowcaseSlot & { dirty?: boolean };

export function normalizeShowcaseSlots(items: ShowcaseSlot[] = []): DraftSlot[] {
  const bySlot = new Map(items.map((item) => [item.slot_no, item]));
  return Array.from({ length: 12 }, (_, index) => {
    const slotNo = index + 1;
    const source = bySlot.get(slotNo);
    if (!source || source.product_id <= 0) {
      return { slot_no: slotNo, product_id: 0, empty: true, valid: true };
    }
    return { ...source, slot_no: slotNo, empty: false };
  });
}

export function moveSlot(items: DraftSlot[], from: number, to: number): DraftSlot[] {
  if (from === to || from < 0 || to < 0 || from >= items.length || to >= items.length) return items;
  const next = [...items];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next.map((slot, index) => ({ ...slot, slot_no: index + 1, dirty: true }));
}

const invalidReasonText: Record<string, string> = {
  product_not_found: '商品不存在',
  product_inactive: '商品已下架',
  merchant_not_found: '所属商家不存在',
  merchant_inactive: '所属店铺已停用',
  out_of_stock: '库存不足或已售罄',
};

function candidateDisabledReason(candidate: ShowcaseCandidate, slots: DraftSlot[]): string {
  if (slots.some((slot) => slot.product_id === candidate.product.product_id)) return '该商品已在橱窗中';
  const merchantId = candidate.product.merchant_id;
  if (merchantId && slots.filter((slot) => slot.product?.merchant_id === merchantId).length >= 2) {
    return '同一商家最多展示 2 件商品';
  }
  if (!slots.some((slot) => slot.product_id <= 0)) return '橱窗已满';
  return '';
}

export default function ShowcasePage() {
  const [slots, setSlots] = useState<DraftSlot[]>(() => normalizeShowcaseSlots());
  const [candidates, setCandidates] = useState<ShowcaseCandidate[]>([]);
  const [version, setVersion] = useState(0);
  const [publishTime, setPublishTime] = useState('');
  const [loading, setLoading] = useState(true);
  const [publishing, setPublishing] = useState(false);
  const [dragFrom, setDragFrom] = useState<number | null>(null);
  const [notice, setNotice] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const load = async () => {
    setLoading(true);
    setNotice(null);
    try {
      const [layoutResponse, candidatesResponse] = await Promise.all([
        authed<ShowcaseResp>('/api/admin/showcase'),
        authed<ShowcaseCandidatesResp>('/api/admin/showcase/candidates?page=1&page_size=50'),
      ]);
      if (!layoutResponse.ok || !candidatesResponse.ok) {
        setNotice({ type: 'error', text: '橱窗数据加载失败，请稍后重试' });
        return;
      }
      setSlots(normalizeShowcaseSlots(layoutResponse.data.items));
      setVersion(layoutResponse.data.version || 0);
      setPublishTime(layoutResponse.data.publish_time || '');
      setCandidates(candidatesResponse.data.items || []);
    } catch {
      setNotice({ type: 'error', text: '橱窗数据加载失败，请稍后重试' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  const merchantCounts = useMemo(() => {
    const counts = new Map<number, number>();
    slots.forEach((slot) => {
      const merchantId = slot.product?.merchant_id;
      if (merchantId) counts.set(merchantId, (counts.get(merchantId) || 0) + 1);
    });
    return counts;
  }, [slots]);

  const addCandidate = (candidate: ShowcaseCandidate) => {
    if (candidateDisabledReason(candidate, slots)) return;
    const emptyIndex = slots.findIndex((slot) => slot.product_id <= 0);
    if (emptyIndex < 0) return;
    setSlots((current) => current.map((slot, index) => index === emptyIndex ? {
      slot_no: slot.slot_no,
      product_id: candidate.product.product_id,
      product: candidate.product,
      empty: false,
      valid: true,
      dirty: true,
    } : slot));
  };

  const removeSlot = (index: number) => {
    setSlots((current) => current.map((slot, slotIndex) => slotIndex === index ? {
      slot_no: slot.slot_no,
      product_id: 0,
      empty: true,
      valid: true,
      dirty: true,
    } : slot));
  };

  const publish = async () => {
    const payload: ShowcasePublishReq = {
      expected_version: version,
      items: slots
        .filter(({ product_id: productId }) => productId > 0)
        .map(({ slot_no: slotNo, product_id: productId }) => ({ slot_no: slotNo, product_id: productId })),
    };
    setPublishing(true);
    setNotice(null);
    try {
      const response = await authed<ShowcaseResp>('/api/admin/showcase/publish', {
        method: 'POST',
        jsonBody: payload,
      });
      if (response.status === 409) {
        setNotice({ type: 'error', text: '橱窗已被其他管理员更新，请刷新后重试' });
        return;
      }
      if (!response.ok) {
        setNotice({ type: 'error', text: '橱窗发布失败，请检查失效商品后重试' });
        return;
      }
      setSlots(normalizeShowcaseSlots(response.data.items));
      setVersion(response.data.version);
      setPublishTime(response.data.publish_time || '刚刚');
      setNotice({ type: 'success', text: `橱窗已发布，当前版本 ${response.data.version}` });
    } catch {
      setNotice({ type: 'error', text: '橱窗发布失败，请稍后重试' });
    } finally {
      setPublishing(false);
    }
  };

  if (loading) return <div style={{ minHeight: 420, display: 'grid', placeItems: 'center' }}><Spin size="large" /></div>;

  return (
    <div style={{ maxWidth: 1320, margin: '0 auto' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 20, marginBottom: 18 }}>
        <div>
          <h1 style={{ margin: 0 }}>首页橱窗</h1>
          <p style={{ margin: '6px 0 0', color: '#7b7b7b' }}>人工控制首页展示顺序，推荐分只提供决策依据。</p>
        </div>
        <Space wrap>
          <Tag color="blue">版本 {version}</Tag>
          {publishTime && <span style={{ color: '#8c8c8c', fontSize: 12 }}>上次发布：{publishTime}</span>}
          <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
          <Button type="primary" icon={<SendOutlined />} loading={publishing} onClick={publish}>发布首页橱窗</Button>
        </Space>
      </div>

      {notice && <Alert type={notice.type} showIcon message={notice.text} style={{ marginBottom: 18 }} />}

      <Row gutter={[18, 18]} align="top">
        <Col xs={24} xl={15}>
          <Card title="12 个首页槽位" extra={<span style={{ color: '#8c8c8c' }}>{slots.filter((slot) => slot.product_id > 0).length} / 12 已使用</span>}>
            <div style={{ display: 'grid', gap: 10 }}>
              {slots.map((slot, index) => {
                const imageURL = slot.product ? resolveProductImage(slot.product.product_id, slot.product.image_url) : '';
                return (
                  <div
                    key={slot.slot_no}
                    data-testid={`showcase-slot-${slot.slot_no}`}
                    draggable={slot.product_id > 0}
                    onDragStart={() => setDragFrom(index)}
                    onDragOver={(event) => event.preventDefault()}
                    onDrop={() => {
                      if (dragFrom !== null) setSlots((current) => moveSlot(current, dragFrom, index));
                      setDragFrom(null);
                    }}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '44px 62px minmax(0,1fr) auto',
                      alignItems: 'center',
                      gap: 12,
                      minHeight: 76,
                      padding: 10,
                      border: `1px ${slot.product_id > 0 ? 'solid' : 'dashed'} ${slot.valid ? '#e5e5e5' : '#ff9c6e'}`,
                      borderRadius: 10,
                      background: slot.valid ? '#fff' : '#fff7e6',
                    }}
                  >
                    <strong style={{ color: '#b23a2c', textAlign: 'center' }}>{String(slot.slot_no).padStart(2, '0')}</strong>
                    <div style={{ width: 62, height: 54, display: 'grid', placeItems: 'center', overflow: 'hidden', borderRadius: 8, background: '#f7efe8', color: '#b23a2c' }}>
                      {imageURL ? <img src={imageURL} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : slot.product_id > 0 ? '商品' : '＋'}
                    </div>
                    {slot.product_id > 0 ? (
                      <div style={{ minWidth: 0 }}>
                        <strong style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{slot.product?.name || `商品 ${slot.product_id}`}</strong>
                        <Space size={8} wrap style={{ marginTop: 5, fontSize: 12, color: '#8c8c8c' }}>
                          <span>{slot.product?.merchant_name || `商品 ID ${slot.product_id}`}</span>
                          {slot.product && <span>¥{formatPriceFen(slot.product.final_price_fen)}</span>}
                          {!slot.valid && <Tag color="error">{invalidReasonText[slot.invalid_reason || ''] || '当前不可展示'}</Tag>}
                        </Space>
                      </div>
                    ) : (
                      <div><strong>空槽位</strong><div style={{ color: '#aaa', fontSize: 12, marginTop: 4 }}>可从右侧候选加入</div></div>
                    )}
                    <Space size={2}>
                      <Tooltip title="上移">
                        <Button aria-label="上移" size="small" icon={<ArrowUpOutlined />} disabled={index === 0} onClick={() => setSlots((current) => moveSlot(current, index, index - 1))} />
                      </Tooltip>
                      <Tooltip title="下移">
                        <Button aria-label="下移" size="small" icon={<ArrowDownOutlined />} disabled={index === slots.length - 1} onClick={() => setSlots((current) => moveSlot(current, index, index + 1))} />
                      </Tooltip>
                      <Tooltip title="移除">
                        <Button aria-label="移除" size="small" danger icon={<DeleteOutlined />} disabled={slot.product_id <= 0} onClick={() => removeSlot(index)} />
                      </Tooltip>
                    </Space>
                  </div>
                );
              })}
            </div>
          </Card>
        </Col>

        <Col xs={24} xl={9}>
          <Card
            title="推荐候选"
            extra={<Tag color="gold">仅建议，不会自动发布</Tag>}
            styles={{ body: { maxHeight: 'calc(100vh - 220px)', overflowY: 'auto' } }}
          >
            <div style={{ display: 'grid', gap: 14 }}>
              {candidates.map((candidate) => {
                const reason = candidateDisabledReason(candidate, slots);
                const merchantCount = candidate.product.merchant_id ? merchantCounts.get(candidate.product.merchant_id) || 0 : 0;
                return (
                  <div key={candidate.product.product_id} style={{ border: '1px solid #eee', borderRadius: 12, padding: 14 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
                      <div style={{ minWidth: 0 }}>
                        <strong>{candidate.product.name}</strong>
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
                      <Button
                        block
                        type="primary"
                        ghost
                        icon={<PlusOutlined />}
                        disabled={Boolean(reason)}
                        aria-label={`加入${candidate.product.name}`}
                        onClick={() => addCandidate(candidate)}
                        style={{ marginTop: 10 }}
                      >
                        {reason || '加入第一个空槽'}
                      </Button>
                    </Tooltip>
                  </div>
                );
              })}
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
