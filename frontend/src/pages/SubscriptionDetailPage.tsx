// pages/SubscriptionDetailPage.tsx — 订阅详情页（可编辑、删除、查看走势）
// 已适配后端 PDF 8 表重构：
//   - 影院级订阅：cinema_name + email + target_price + notify_enabled + status
//   - 去掉 movie_name / city_name / score 等旧字段
//   - 日志字段：matched_price / notify_status / error_msg / sent_at
import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  Card, Table, Tag, Button, message, Typography, Space, Empty, Spin,
  Modal, Form, Input, InputNumber, Switch, Breadcrumb,
} from 'antd'
import {
  EditOutlined, DeleteOutlined, ArrowLeftOutlined,
  PlayCircleOutlined, StopOutlined, HomeOutlined, BellOutlined,
} from '@ant-design/icons'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import { useAuth } from '../store/auth'
import {
  getSubscriptionDetail, updateSubscription, deleteSubscription,
  exportSubscriptionCSV,
} from '../api/endpoints'

const { Title, Text } = Typography

interface PricePoint { time: string; price: number }

// 匹配后端 ShowPriceForSubscription
interface ShowItem {
  movie_name: string; cinema_name: string;
  hall_name: string; show_date: string; show_time: string;
  current_price: number; vip_price?: number;
  base_price?: number; discount_price?: number;
}

// 匹配后端 SubscriptionDetail
interface SubDetail {
  subscription: {
    id: number
    cinema_id: number
    email: string
    target_price: number
    notify_enabled: boolean
    status: number             // 0=停用, 1=启用
    baseline_min_price?: number
    baseline_max_price?: number
    last_notify_at?: string
    notify_count: number
    remark?: string
    created_at: string
    updated_at: string
    cinema_name?: string
    cinema_address?: string
  }
  current_shows: ShowItem[]
  price_trend: PricePoint[]
  history_total: number
}

export default function SubscriptionDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { user } = useAuth()
  const navigate = useNavigate()

  const [detail, setDetail] = useState<SubDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [editOpen, setEditOpen] = useState(false)
  const [editForm] = Form.useForm()
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [exporting, setExporting] = useState(false)

  const loadDetail = useCallback(async () => {
    if (!id) return
    setLoading(true)
    try {
      const res = await getSubscriptionDetail(id)
      if (res.code === 0) setDetail(res.data)
      else message.error('加载订阅详情失败')
    } catch { message.error('加载订阅详情失败') }
    finally { setLoading(false) }
  }, [id])

  useEffect(() => { if (user) loadDetail() }, [user, loadDetail])

  const handleEditOpen = () => {
    if (!detail) return
    editForm.setFieldsValue({
      email: detail.subscription.email,
      target_price: detail.subscription.target_price || undefined,
      notify_enabled: detail.subscription.notify_enabled,
      remark: detail.subscription.remark || '',
    })
    setEditOpen(true)
  }

  const handleEditSave = async () => {
    if (!id) return
    try {
      const values = await editForm.validateFields()
      const data: any = {}
      if (values.email != null) data.email = values.email
      if (values.target_price != null) data.target_price = Number(values.target_price)
      if (values.notify_enabled != null) data.notify_enabled = values.notify_enabled
      if (values.remark != null) data.remark = values.remark
      setSaving(true)
      const res = await updateSubscription(id, data)
      if (res.code === 0) {
        message.success('更新成功')
        setEditOpen(false)
        loadDetail()
      } else {
        message.error(res.msg || '更新失败')
      }
    } catch {
      message.error('更新失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = () => {
    if (!id) return
    Modal.confirm({
      title: '确认删除',
      content: '删除后无法恢复，是否继续？',
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setDeleting(true)
        try {
          const res = await deleteSubscription(id)
          if (res.code === 0) {
            message.success('已删除')
            navigate('/subscriptions')
          } else {
            message.error(res.msg || '删除失败')
          }
        } catch { message.error('删除失败') }
        finally { setDeleting(false) }
      },
    })
  }

  const handleExport = async () => {
    if (!id) return
    setExporting(true)
    try {
      const res = await exportSubscriptionCSV(id)
      const url = window.URL.createObjectURL(res as any)
      const a = document.createElement('a')
      a.href = url
      a.download = `subscription_${id}_${new Date().toISOString().slice(0, 10)}.csv`
      a.click()
      window.URL.revokeObjectURL(url)
      message.success('导出成功')
    } catch { message.error('导出失败') }
    finally { setExporting(false) }
  }

  // 表格列定义（匹配后端 ShowPriceForSubscription）
  const showColumns: any[] = [
    { title: '电影', dataIndex: 'movie_name', width: 120 },
    { title: '影厅', dataIndex: 'hall_name', width: 75 },
    { title: '日期', dataIndex: 'show_date', width: 100,
      sorter: (a: ShowItem, b: ShowItem) => (a.show_date || '').localeCompare(b.show_date || ''),
    },
    {
      title: '开场', dataIndex: 'show_time', width: 75,
      sorter: (a: ShowItem, b: ShowItem) => (a.show_time || '').localeCompare(b.show_time || ''),
    },
    {
      title: '票价', dataIndex: 'current_price', width: 80,
      render: (v: number) => {
        const price = v ?? 0
        const allPrices = detail?.current_shows?.map(s => s.current_price ?? 0).filter(p => p > 0) || []
        const isLowest = price > 0 && allPrices.length > 0 && price === Math.min(...allPrices)
        return <Text strong style={{ color: isLowest ? '#e54847' : '#fff', fontSize: 13 }}>
          ¥{price.toFixed(1)}
        </Text>
      },
      sorter: (a: ShowItem, b: ShowItem) => (a.current_price ?? 0) - (b.current_price ?? 0),
    },
    {
      title: '会员', dataIndex: 'vip_price', width: 75,
      render: (v?: number) => v && v > 0 ? <Text type="secondary" style={{ fontSize: 12 }}>¥{v.toFixed(1)}</Text> : '-',
    },
    {
      title: '基础价', dataIndex: 'base_price', width: 80,
      render: (v?: number) => v && v > 0 ? <Text type="secondary" style={{ fontSize: 12 }}>¥{v.toFixed(1)}</Text> : '-',
    },
    {
      title: '优惠价', dataIndex: 'discount_price', width: 85,
      render: (v?: number) => v && v > 0
        ? <Tag color="orange" style={{ margin: 0, fontSize: 12 }}>¥{v.toFixed(1)}</Tag>
        : '-',
    },
  ]

  if (!user) {
    return (
      <div style={{ maxWidth: 600, margin: '40px auto', padding: '0 16px' }}>
        <Card><Empty description="请先登录" /></Card>
      </div>
    )
  }

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Spin size="large"><Text style={{ color: '#fff' }}>加载中...</Text></Spin>
      </div>
    )
  }

  if (!detail) {
    return (
      <div style={{ maxWidth: 600, margin: '40px auto', padding: '0 16px' }}>
        <Card><Empty description="订阅不存在" /></Card>
      </div>
    )
  }

  const sub = detail.subscription

  return (
    <div style={{ minHeight: '100vh', background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)', padding: '24px' }}>
      <div style={{ maxWidth: 1200, margin: '0 auto' }}>
        {/* 面包屑 */}
        <Breadcrumb
          items={[
            { title: <a href="/"><HomeOutlined /> 首页</a> },
            { title: <a href="/subscriptions">订阅管理</a> },
            { title: sub.cinema_name || `影院#${sub.cinema_id}` },
          ]}
          style={{ marginBottom: 20 }}
        />

        {/* 订阅信息卡片 */}
        <Card
          style={{
            borderRadius: 12, marginBottom: 24,
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid rgba(255,255,255,0.08)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <div>
              <Title level={3} style={{ color: '#fff', margin: 0, marginBottom: 12 }}>
                <BellOutlined style={{ marginRight: 8 }} />{sub.cinema_name || `影院#${sub.cinema_id}`}
              </Title>
              <Space direction="vertical" size={4}>
                {sub.cinema_address && (
                  <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                    地址：<Text strong style={{ color: '#fff' }}>{sub.cinema_address}</Text>
                  </Text>
                )}
                <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                  目标价：{sub.target_price > 0 ? <Tag color="red">¥{sub.target_price.toFixed(1)}</Tag> : <Tag>无</Tag>}
                </Text>
                <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                  通知邮箱：<Text style={{ color: '#fff' }}>{sub.email}</Text>
                </Text>
                <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                  通知开关：{sub.notify_enabled
                    ? <Tag color="blue">开启</Tag>
                    : <Tag>关闭</Tag>}
                </Text>
                <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                  状态：{sub.status === 1
                    ? <Tag icon={<PlayCircleOutlined />} color="green">监控中</Tag>
                    : <Tag icon={<StopOutlined />} color="default">已停用</Tag>}
                </Text>
                {sub.baseline_min_price != null && (
                  <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                    基准价：<Tag color="cyan">¥{sub.baseline_min_price.toFixed(1)}</Tag>
                  </Text>
                )}
                <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                  通知次数：<Text strong style={{ color: '#fff' }}>{sub.notify_count}</Text>
                </Text>
                {sub.last_notify_at && (
                  <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                    上次通知：<Text style={{ color: '#fff' }}>{new Date(sub.last_notify_at).toLocaleString()}</Text>
                  </Text>
                )}
                {sub.remark && (
                  <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
                    备注：<Text style={{ color: '#fff' }}>{sub.remark}</Text>
                  </Text>
                )}
                <Text type="secondary" style={{ fontSize: 12 }}>
                  创建于 {new Date(sub.created_at).toLocaleString()}
                </Text>
              </Space>
            </div>
            <Space direction="vertical">
              <Button
                icon={<EditOutlined />}
                onClick={handleEditOpen}
                style={{ borderRadius: 8 }}
              >
                编辑
              </Button>
              <Button
                danger
                icon={<DeleteOutlined />}
                onClick={handleDelete}
                loading={deleting}
                style={{ borderRadius: 8 }}
              >
                删除
              </Button>
              <Button
                icon={<ArrowLeftOutlined />}
                onClick={() => navigate('/subscriptions')}
                style={{ borderRadius: 8 }}
              >
                返回
              </Button>
            </Space>
          </div>
        </Card>

        {/* 价格走势折线图 */}
        {detail.price_trend && detail.price_trend.length > 1 && (
          <Card
            title={<Text strong style={{ color: '#fff', fontSize: 16 }}>票价走势</Text>}
            style={{
              borderRadius: 12, marginBottom: 24,
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.08)',
            }}
          >
            <div style={{ width: '100%', height: 300 }}>
              <ResponsiveContainer>
                <LineChart data={detail.price_trend}>
                  <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.08)" />
                  <XAxis dataKey="time" fontSize={11} stroke="rgba(255,255,255,0.45)" />
                  <YAxis fontSize={11} stroke="rgba(255,255,255,0.45)" />
                  <Tooltip
                    contentStyle={{ fontSize: 12, borderRadius: 8, background: '#1a1a2e', border: '1px solid rgba(255,255,255,0.12)' }}
                    formatter={(v: number) => [`¥${v}`, '最低票价']}
                  />
                  <Line type="monotone" dataKey="price" stroke="#e54847" strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </Card>
        )}

        {/* 当前行情表格 */}
        {detail.current_shows && detail.current_shows.length > 0 ? (
          <Card
            title={<Text strong style={{ color: '#fff', fontSize: 16 }}>当前行情（共 {detail.history_total} 条记录）</Text>}
            extra={
              <Button
                icon={<ArrowLeftOutlined />}
                onClick={handleExport}
                loading={exporting}
                size="small"
                style={{ borderRadius: 6 }}
              >
                导出 CSV
              </Button>
            }
            style={{
              borderRadius: 12,
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.08)',
            }}
          >
            <Table
              columns={showColumns}
              dataSource={detail.current_shows}
              rowKey={(_, i) => String(i)}
              pagination={false}
              size="small"
              showSorterTooltip={false}
              scroll={{ x: 900 }}
            />
          </Card>
        ) : (
          <Card
            style={{
              borderRadius: 12,
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.08)',
            }}
          >
            <Empty description={<Text style={{ color: '#999' }}>暂无行情数据</Text>} />
          </Card>
        )}

        {/* 编辑 Modal */}
        <Modal
          title="编辑订阅"
          open={editOpen}
          onOk={handleEditSave}
          onCancel={() => setEditOpen(false)}
          confirmLoading={saving}
          okText="保存"
          cancelText="取消"
        >
          <Form form={editForm} layout="vertical" style={{ marginTop: 16 }}>
            <Form.Item
              label="通知邮箱"
              name="email"
              rules={[
                { required: true, message: '请输入邮箱' },
                { type: 'email', message: '请输入有效邮箱' },
              ]}
            >
              <Input placeholder="通知邮箱地址" />
            </Form.Item>
            <Form.Item label="目标票价" name="target_price">
              <InputNumber prefix="¥" placeholder="例如 35.0" min={0} step={0.1} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item label="备注" name="remark">
              <Input.TextArea placeholder="备注信息" rows={2} />
            </Form.Item>
            <Form.Item label="通知开关" name="notify_enabled" valuePropName="checked">
              <Switch checkedChildren="开启" unCheckedChildren="关闭" />
            </Form.Item>
          </Form>
        </Modal>
      </div>
    </div>
  )
}
