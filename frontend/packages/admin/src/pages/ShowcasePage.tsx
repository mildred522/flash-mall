import { useEffect, useState } from 'react';
import { ReloadOutlined, SendOutlined } from '@ant-design/icons';
import { Alert, Button, Col, Row, Space, Spin, Tag } from 'antd';
import { authed } from '@flash-mall/shared';
import type { ShowcaseCandidate, ShowcaseCandidatesResp, ShowcasePublishReq, ShowcaseResp } from '@flash-mall/shared';
import ShowcaseCandidates from '../components/showcase/ShowcaseCandidates';
import ShowcaseSlots from '../components/showcase/ShowcaseSlots';
import {
  addShowcaseCandidate, moveSlot, normalizeShowcaseSlots, removeShowcaseSlot, type DraftSlot,
} from '../components/showcase/showcaseModel';

export { moveSlot, normalizeShowcaseSlots } from '../components/showcase/showcaseModel';

export default function ShowcasePage() {
  const [slots, setSlots] = useState<DraftSlot[]>(() => normalizeShowcaseSlots());
  const [candidates, setCandidates] = useState<ShowcaseCandidate[]>([]);
  const [version, setVersion] = useState(0);
  const [publishTime, setPublishTime] = useState('');
  const [loading, setLoading] = useState(true);
  const [publishing, setPublishing] = useState(false);
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

  const publish = async () => {
    const payload: ShowcasePublishReq = {
      expected_version: version,
      items: slots.filter((slot) => slot.product_id > 0)
        .map((slot) => ({ slot_no: slot.slot_no, product_id: slot.product_id })),
    };
    setPublishing(true);
    setNotice(null);
    try {
      const response = await authed<ShowcaseResp>('/api/admin/showcase/publish', {
        method: 'POST', jsonBody: payload,
      });
      if (response.status === 409) {
        setNotice({ type: 'error', text: '橱窗已被其他管理员更新，请刷新后重试' });
      } else if (!response.ok) {
        setNotice({ type: 'error', text: '橱窗发布失败，请检查失效商品后重试' });
      } else {
        setSlots(normalizeShowcaseSlots(response.data.items));
        setVersion(response.data.version);
        setPublishTime(response.data.publish_time || '刚刚');
        setNotice({ type: 'success', text: `橱窗已发布，当前版本 ${response.data.version}` });
      }
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
        <div><h1 style={{ margin: 0 }}>首页橱窗</h1>
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
          <ShowcaseSlots slots={slots}
            onMove={(from, to) => setSlots((current) => moveSlot(current, from, to))}
            onRemove={(index) => setSlots((current) => removeShowcaseSlot(current, index))} />
        </Col>
        <Col xs={24} xl={9}>
          <ShowcaseCandidates candidates={candidates} slots={slots}
            onAdd={(candidate) => setSlots((current) => addShowcaseCandidate(current, candidate))} />
        </Col>
      </Row>
    </div>
  );
}
