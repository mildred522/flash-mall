import { ReloadOutlined } from '@ant-design/icons';
import { Button, Input, Select, Space, Tag } from 'antd';

type Props = {
  userId: string;
  result?: string;
  eventType?: string;
  keyword: string;
  eventOptions: Array<{ value: string; label: string }>;
  filteredCount: number;
  totalCount: number;
  onUserIdChange: (value: string) => void;
  onResultChange: (value?: string) => void;
  onEventTypeChange: (value?: string) => void;
  onKeywordChange: (value: string) => void;
  onReload: () => void;
};

export default function SecurityEventFilters(props: Props) {
  return (
    <Space wrap>
      <Input allowClear inputMode="numeric" placeholder="用户ID" style={{ width: 120 }} value={props.userId}
        onChange={(event) => props.onUserIdChange(event.target.value)} />
      <Select allowClear placeholder="事件" style={{ width: 180 }} options={props.eventOptions}
        value={props.eventType} onChange={props.onEventTypeChange} />
      <Select allowClear placeholder="结果" style={{ width: 120 }} value={props.result}
        options={[
          { value: 'success', label: '成功' }, { value: 'blocked', label: '拦截' },
          { value: 'fail', label: '失败' }, { value: 'failed', label: '失败' },
        ]}
        onChange={props.onResultChange} />
      <Input allowClear placeholder="用户ID/主体/IP/事件" style={{ width: 180 }} value={props.keyword}
        onChange={(event) => props.onKeywordChange(event.target.value)} />
      <Button icon={<ReloadOutlined />} onClick={props.onReload}>刷新</Button>
      <Tag>{props.filteredCount}/{props.totalCount}</Tag>
    </Space>
  );
}
