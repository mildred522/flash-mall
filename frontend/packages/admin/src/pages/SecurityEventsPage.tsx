import { useEffect, useMemo, useState } from 'react';
import { Table, message } from 'antd';
import { authed } from '@flash-mall/shared';
import type { SecurityEventItem, SecurityEventsRecentResp } from '@flash-mall/shared';
import SecurityEventFilters from '../components/security/SecurityEventFilters';
import { createSecurityEventColumns } from '../components/security/securityEventColumns';
import { eventOptions, filterSecurityEvents } from '../components/security/securityEventModel';

export default function SecurityEventsPage() {
  const [items, setItems] = useState<SecurityEventItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [userId, setUserId] = useState(() => {
    const initialUserId = window.__flashAdminSecurityUserId || 0;
    window.__flashAdminSecurityUserId = 0;
    return initialUserId ? String(initialUserId) : '';
  });
  const [result, setResult] = useState<string>();
  const [eventType, setEventType] = useState<string>();
  const [keyword, setKeyword] = useState(() => {
    const initialKeyword = window.__flashAdminSecurityKeyword || '';
    window.__flashAdminSecurityKeyword = '';
    return initialKeyword;
  });

  const load = async () => {
    setLoading(true);
    try {
      const query = new URLSearchParams({ limit: '100' });
      if (userId) query.set('user_id', userId);
      if (result) query.set('result', result);
      if (eventType) query.set('event_type', eventType);
      if (keyword.trim()) query.set('keyword', keyword.trim());
      const response = await authed<SecurityEventsRecentResp>(`/api/admin/security/events/recent?${query}`);
      if (response.ok && !response.data.error) {
        setItems(response.data.items || []);
      } else {
        message.error(response.data.error || '安全日志加载失败');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  const options = useMemo(() => eventOptions(items), [items]);
  const filteredItems = useMemo(
    () => filterSecurityEvents(items, userId, result, eventType, keyword),
    [eventType, items, keyword, result, userId],
  );
  const columns = useMemo(() => createSecurityEventColumns(), []);

  return (
    <Table<SecurityEventItem>
      rowKey={(row) => `${row.created_at}-${row.user_id}-${row.event_type}-${row.subject}`}
      loading={loading}
      dataSource={filteredItems}
      size="small"
      pagination={false}
      title={() => (
        <SecurityEventFilters
          userId={userId}
          result={result}
          eventType={eventType}
          keyword={keyword}
          eventOptions={options}
          filteredCount={filteredItems.length}
          totalCount={items.length}
          onUserIdChange={setUserId}
          onResultChange={setResult}
          onEventTypeChange={setEventType}
          onKeywordChange={setKeyword}
          onReload={() => void load()}
        />
      )}
      columns={columns}
    />
  );
}
