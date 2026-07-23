import { Button, Tag, type TableProps } from 'antd';
import type { SecurityEventItem } from '@flash-mall/shared';
import { eventText, formatEventTime, reasonText, subjectNavigation } from './securityEventModel';

function resultTag(result: string) {
  if (result === 'success') return <Tag color="green">成功</Tag>;
  if (result === 'blocked') return <Tag color="orange">拦截</Tag>;
  if (result === 'fail' || result === 'failed') return <Tag color="red">失败</Tag>;
  return <Tag>{result || '-'}</Tag>;
}

function navigate(detail: object) {
  window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail }));
}

export function createSecurityEventColumns(): TableProps<SecurityEventItem>['columns'] {
  return [
    { title: '时间', dataIndex: 'created_at', width: 180, render: (_, row) => formatEventTime(row.created_at) },
    { title: '事件', dataIndex: 'event_type', ellipsis: true, render: (_, row) => eventText(row.event_type) },
    { title: '结果', dataIndex: 'result', width: 90, render: (_, row) => resultTag(row.result) },
    { title: '用户ID', dataIndex: 'user_id', width: 100, render: (_, row) => row.user_id ? (
      <Button type="link" size="small" onClick={() => navigate({ path: '/admin/users', userId: row.user_id })}>
        {row.user_id}
      </Button>
    ) : '-' },
    { title: '主体', dataIndex: 'subject', ellipsis: true, render: (_, row) => {
      const subject = row.subject || '';
      const navigation = subjectNavigation(subject);
      if (!subject) return '-';
      return navigation
        ? <Button type="link" size="small" onClick={() => navigate(navigation)}>{subject}</Button>
        : subject;
    } },
    { title: 'IP', dataIndex: 'ip', width: 150, render: (_, row) => row.ip || '-' },
    { title: '原因', dataIndex: 'subject', width: 160, render: (_, row) => reasonText(row.subject) },
    { title: 'User-Agent', dataIndex: 'user_agent', width: 220, ellipsis: true, render: (_, row) => row.user_agent || '-' },
  ];
}
