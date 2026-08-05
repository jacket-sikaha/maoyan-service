// pages/SubscriptionHistoryPage.tsx — 订阅通知日志页（需登录）
// 已适配后端 PDF 8 表重构：
//   - 日志字段：notify_type / notify_status / target_price / matched_price / error_msg / sent_at
//   - SubscriptionLogFullInfo: cinema_name / email / notify_type / notify_status / target_price / matched_price / error_msg / sent_at
import { useState, useEffect, useCallback } from 'react'
import {
  Card, Table, Tag, Typography, Empty, DatePicker, Space, Select, message,
} from 'antd'
import { HistoryOutlined } from '@ant-design/icons'
import { useAuth } from '../store/auth'
import { getSubscriptionLogs } from '../api/endpoints'

const { Title, Text } = Typography
const { RangePicker } = DatePicker

// 匹配后端 SubscriptionLogFullInfo
interface LogItem {
  id: number
  subscription_id: number
  cinema_name: string
  email: string
  notify_type: string       // price_alert
  notify_status: string     // success / fail
  target_price: number
  matched_price: number
  error_msg?: string
  sent_at?: string
  created_at: string
}

export default function SubscriptionHistoryPage() {
  const { user } = useAuth()

  const [logs, setLogs] = useState<LogItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [dateRange, setDateRange] = useState<[string, string] | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('')

  const fetchLogs = useCallback(async (p: number, ps: number, dates: [string, string] | null, status: string) => {
    setLoading(true)
    try {
      const params: any = { page: p, page_size: ps }
      if (dates) {
        params.start_date = dates[0]
        params.end_date = dates[1]
      }
      if (status) {
        params.status = status
      }
      const res = await getSubscriptionLogs(params)
      if (res.code === 0) {
        setLogs(res.data.items || [])
        setTotal(res.data.total || 0)
      } else {
        message.error('加载日志失败')
      }
    } catch {
      message.error('加载日志失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (user) fetchLogs(page, pageSize, dateRange, statusFilter)
  }, [user, page, pageSize, dateRange, statusFilter, fetchLogs])

  const handleDateChange = (_: any, dateStrings: [string, string]) => {
    if (dateStrings[0] && dateStrings[1]) {
      setDateRange(dateStrings)
      setPage(1)
    } else {
      setDateRange(null)
      setPage(1)
    }
  }

  const handleStatusChange = (value: string) => {
    setStatusFilter(value || '')
    setPage(1)
  }

  const columns = [
    {
      title: '影院',
      key: 'cinema',
      render: (_: any, item: LogItem) => (
        <div>
          <Text strong style={{ color: '#fff' }}>{item.cinema_name || `影院#${item.subscription_id}`}</Text>
          <br />
          <Text type="secondary" style={{ fontSize: 12 }}>{item.email}</Text>
        </div>
      ),
    },
    {
      title: '通知类型',
      dataIndex: 'notify_type',
      key: 'notify_type',
      width: 100,
      render: (v: string) => {
        const map: Record<string, { color: string; text: string }> = {
          price_alert: { color: 'orange', text: '降价提醒' },
          baseline: { color: 'blue', text: '基准价' },
        }
        const s = map[v] || { color: 'default', text: v || '-' }
        return <Tag color={s.color}>{s.text}</Tag>
      },
    },
    {
      title: '目标价',
      dataIndex: 'target_price',
      key: 'target_price',
      width: 90,
      render: (v: number) => v > 0 ? <Tag color="red">¥{v.toFixed(1)}</Tag> : <Tag>无</Tag>,
    },
    {
      title: '命中价',
      dataIndex: 'matched_price',
      key: 'matched_price',
      width: 90,
      render: (v: number) => v > 0 ? <Tag color="green">¥{v.toFixed(1)}</Tag> : <Text type="secondary">-</Text>,
    },
    {
      title: '通知状态',
      dataIndex: 'notify_status',
      key: 'notify_status',
      width: 100,
      render: (v: string) => {
        const map: Record<string, { color: string; text: string }> = {
          success: { color: 'green', text: '成功' },
          fail: { color: 'red', text: '失败' },
          failed: { color: 'red', text: '失败' },
          pending: { color: 'default', text: '待处理' },
        }
        const s = map[v] || { color: 'default', text: v || '-' }
        return <Tag color={s.color}>{s.text}</Tag>
      },
    },
    {
      title: '错误信息',
      dataIndex: 'error_msg',
      key: 'error_msg',
      ellipsis: true,
      render: (v?: string) => (
        <Text style={{ color: 'rgba(255,255,255,0.65)', maxWidth: 300, display: 'inline-block' }}>
          {v || '-'}
        </Text>
      ),
    },
    {
      title: '发送时间',
      dataIndex: 'sent_at',
      key: 'sent_at',
      width: 180,
      render: (v?: string) => (
        <Text style={{ color: 'rgba(255,255,255,0.65)', fontSize: 13 }}>
          {v ? new Date(v).toLocaleString() : '-'}
        </Text>
      ),
    },
  ]

  if (!user) {
    return (
      <div style={{ maxWidth: 600, margin: '40px auto', padding: '0 16px' }}>
        <Card><Empty description="请先登录" /></Card>
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)', padding: '24px' }}>
      <div style={{ maxWidth: 1200, margin: '0 auto' }}>
        <Title level={3} style={{ color: '#fff', marginBottom: 24 }}>
          <HistoryOutlined style={{ marginRight: 8 }} />通知日志
        </Title>

        {/* 筛选区 */}
        <Card
          size="small"
          style={{
            marginBottom: 16, borderRadius: 12,
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid rgba(255,255,255,0.08)',
          }}
        >
          <Space size="large">
            <Space>
              <Text style={{ color: 'rgba(255,255,255,0.65)' }}>日期范围</Text>
              <RangePicker onChange={handleDateChange} allowClear />
            </Space>
            <Space>
              <Text style={{ color: 'rgba(255,255,255,0.65)' }}>通知状态</Text>
              <Select
                style={{ width: 140 }}
                allowClear
                placeholder="全部状态"
                onChange={handleStatusChange}
                options={[
                  { value: 'success', label: '成功' },
                  { value: 'fail', label: '失败' },
                  { value: 'pending', label: '待处理' },
                  { value: 'skip', label: '跳过' },
                ]}
              />
            </Space>
          </Space>
        </Card>

        {/* 日志表格 */}
        <Card
          style={{
            borderRadius: 12,
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid rgba(255,255,255,0.08)',
          }}
        >
          <Table
            columns={columns}
            dataSource={logs}
            rowKey="id"
            loading={loading}
            pagination={{
              current: page,
              pageSize,
              total,
              showSizeChanger: true,
              showTotal: (t) => `共 ${t} 条日志`,
              onChange: (p, ps) => { setPage(p); setPageSize(ps) },
            }}
            locale={{ emptyText: <Empty description="暂无通知日志" /> }}
          />
        </Card>
      </div>
    </div>
  )
}
