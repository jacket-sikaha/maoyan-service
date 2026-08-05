// pages/SubscriptionsPage.tsx — 订阅管理页（深色主题卡片列表）
// 信息层级：① 影院标识+操作 → ② 核心指标(目标价/最低价/通知) → ③ 底栏次要信息
import { useState, useEffect } from 'react'
import { useAuth } from '../store/auth'
import { listSubscriptions, toggleSubscription, exportSubscriptionCSV } from '../api/endpoints'
import { Card, Tag, Button, Switch, message, Empty, Typography, Space, Tooltip } from 'antd'
import { useNavigate } from 'react-router-dom'
import {
  DownloadOutlined, EditOutlined, BellOutlined,
  MailOutlined, ClockCircleOutlined, EnvironmentOutlined,
  BarChartOutlined,
} from '@ant-design/icons'
import SubscriptionDrawer from '../components/SubscriptionDrawer'

const { Title, Text } = Typography

// 深色主题常量
const BG_PAGE = 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)'
const CARD_BG = 'rgba(255,255,255,0.04)'
const CARD_BORDER = '1px solid rgba(255,255,255,0.08)'
const TEXT_MAIN = '#fff'
const TEXT_SUB = 'rgba(255,255,255,0.65)'
const TEXT_FAINT = 'rgba(255,255,255,0.4)'
const ACCENT_RED = '#e54847'
const ACCENT_GREEN = '#52c41a'
const ACCENT_ORANGE = '#fa8c16'
const ACCENT_BLUE = '#1890ff'

interface SubFull {
  id: number
  cinema_id: number
  movie_id?: string
  movie_name?: string
  email: string
  target_price: number
  initial_target_price: number
  notify_enabled: boolean
  status: number            // 0=停用, 1=启用
  baseline_min_price?: number
  last_notify_at?: string
  notify_count: number
  remark?: string
  created_at: string
  updated_at: string
  cinema_name?: string
  cinema_address?: string
}

// 格式化日期为 M/D
function fmtDate(iso?: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '—'
  return `${d.getMonth() + 1}/${d.getDate()}`
}

export default function SubscriptionsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [subs, setSubs] = useState<SubFull[]>([])
  const [exporting, setExporting] = useState<number | null>(null)
  const [editingSub, setEditingSub] = useState<SubFull | null>(null)
  const [drawerEditOpen, setDrawerEditOpen] = useState(false)

  const loadSubs = async () => {
    const res = await listSubscriptions().catch(() => null)
    if (res?.code === 0) setSubs(res.data || [])
  }

  useEffect(() => { if (user) loadSubs() }, [user])

  const handleToggle = async (sub: SubFull) => {
    const newStatus = sub.status === 1 ? 0 : 1
    try {
      const res = await toggleSubscription(String(sub.id), newStatus)
      if (res.code === 0) {
        message.success(newStatus === 1 ? `已启用「${sub.cinema_name}」` : `已停用「${sub.cinema_name}」`)
        loadSubs()
      } else {
        message.error(res.msg || '操作失败')
      }
    } catch (err: any) {
      message.error(err?.message || '操作失败')
    }
  }

  const handleExport = async (sub: SubFull) => {
    setExporting(sub.id)
    try {
      const res = await exportSubscriptionCSV(String(sub.id))
      const url = window.URL.createObjectURL(res as any)
      const a = document.createElement('a')
      a.href = url; a.download = `subscription_${sub.id}.csv`; a.click()
      window.URL.revokeObjectURL(url)
      message.success('导出成功')
    } catch { message.error('导出失败') }
    finally { setExporting(null) }
  }

  if (!user) return (
    <div style={{ minHeight: '100vh', background: BG_PAGE, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Card style={{ background: CARD_BG, border: CARD_BORDER }}><Empty description={<Text style={{ color: TEXT_SUB }}>请先登录</Text>} /></Card>
    </div>
  )

  const activeCount = subs.filter(s => s.status === 1).length
  const totalNotifyCount = subs.reduce((sum, s) => sum + s.notify_count, 0)

  return (
    <div style={{ minHeight: '100vh', background: BG_PAGE, padding: '24px 16px' }}>
      <div style={{ maxWidth: 960, margin: '0 auto' }}>
        {/* 标题 + 统计 */}
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 16, marginBottom: 28, flexWrap: 'wrap' }}>
          <Title level={3} style={{ color: TEXT_MAIN, margin: 0 }}>
            <BellOutlined style={{ marginRight: 8, color: ACCENT_RED }} />我的订阅
          </Title>
          <Space size={20} style={{ color: TEXT_FAINT, fontSize: 13 }}>
            <span>共 {subs.length} 条</span>
            <span style={{ color: ACCENT_GREEN }}>运行中 {activeCount}</span>
            <span>累计通知 {totalNotifyCount} 次</span>
          </Space>
        </div>

        {/* 空状态 */}
        {subs.length === 0 && (
          <Card style={{ background: CARD_BG, border: CARD_BORDER, borderRadius: 12 }}>
            <Empty description={<Text style={{ color: TEXT_SUB }}>暂无订阅，去首页搜索影院并订阅吧</Text>} />
          </Card>
        )}

        {/* 订阅卡片列表 */}
        {subs.map(sub => {
          const isActive = sub.status === 1
          const hasBaseline = sub.baseline_min_price != null && sub.baseline_min_price > 0
          const cinemaName = sub.cinema_name || `影院#${sub.cinema_id}`
          const movieName = sub.movie_name || ''
          const targetVs = hasBaseline
            ? sub.baseline_min_price! < sub.target_price
              ? { color: TEXT_FAINT, label: '高于最低' }
              : sub.baseline_min_price! === sub.target_price
                ? { color: ACCENT_ORANGE, label: '等于最低' }
                : { color: ACCENT_GREEN, label: '低于最低' }
            : null

          return (
            <Card
              key={sub.id}
              size="small"
              style={{
                background: CARD_BG,
                border: CARD_BORDER,
                borderRadius: 12,
                marginBottom: 12,
              }}
              styles={{ body: { padding: '18px 24px' } }}
              hoverable
            >
              {/* === 第一行：影院标识 + 操作 === */}
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16 }}>
                {/* 左：状态竖条 + 影院名 + 电影/备注 */}
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, flex: 1, minWidth: 0 }}>
                  <div style={{
                    width: 3, height: 36, borderRadius: 2, flexShrink: 0,
                    background: isActive ? ACCENT_GREEN : 'rgba(255,255,255,0.15)',
                  }} />
                  <div style={{ minWidth: 0 }}>
                    <Text
                      strong
                      style={{
                        color: TEXT_MAIN, fontSize: 16,
                        display: 'block',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {cinemaName}
                    </Text>
                    <Space size={6} style={{ marginTop: 2 }}>
                      {movieName && (
                        <Text style={{ color: TEXT_FAINT, fontSize: 12 }}>《{movieName}》</Text>
                      )}
                      {sub.remark && (
                        <Text style={{ color: TEXT_FAINT, fontSize: 12 }}>· {sub.remark}</Text>
                      )}
                    </Space>
                  </div>
                </div>

                {/* 右：操作按钮 */}
                <Space size={2} style={{ flexShrink: 0 }}>
                  <Tooltip title="采集记录">
                    <Button
                      type="text" size="small"
                      icon={<BarChartOutlined />}
                      style={{ color: TEXT_FAINT }}
                      onClick={() => navigate(`/subscription/${sub.id}/records`)}
                    />
                  </Tooltip>
                  <Tooltip title="导出 CSV">
                    <Button
                      type="text" size="small"
                      icon={<DownloadOutlined />}
                      loading={exporting === sub.id}
                      style={{ color: TEXT_FAINT }}
                      onClick={() => handleExport(sub)}
                    />
                  </Tooltip>
                  <Tooltip title="编辑">
                    <Button
                      type="text" size="small"
                      icon={<EditOutlined />}
                      style={{ color: TEXT_FAINT }}
                      onClick={() => { setEditingSub(sub); setDrawerEditOpen(true) }}
                    />
                  </Tooltip>
                </Space>
              </div>

              {/* === 第二行：核心指标 === */}
              <div style={{
                display: 'flex', alignItems: 'center', gap: 20,
                marginTop: 14, paddingTop: 14,
                borderTop: '1px solid rgba(255,255,255,0.06)',
              }}>
                {/* 目标价 — 核心指标，最大视觉权重 */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <Text style={{ color: TEXT_FAINT, fontSize: 11 }}>目标价</Text>
                  {sub.target_price > 0 ? (
                    <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
                      <Text strong style={{ color: ACCENT_RED, fontSize: 20, lineHeight: 1 }}>
                        ¥{sub.target_price.toFixed(1)}
                      </Text>
                      {sub.initial_target_price > 0 && sub.initial_target_price !== sub.target_price && (
                        <Text style={{ color: TEXT_FAINT, fontSize: 11, textDecoration: 'line-through' }}>
                          {sub.initial_target_price.toFixed(1)}
                        </Text>
                      )}
                    </div>
                  ) : (
                    <Text style={{ color: TEXT_FAINT, fontSize: 14, lineHeight: '20px' }}>未设置</Text>
                  )}
                </div>

                {/* 分割线 */}
                <div style={{ width: 1, height: 32, background: 'rgba(255,255,255,0.06)' }} />

                {/* 最低价（baseline_min_price） */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <Text style={{ color: TEXT_FAINT, fontSize: 11 }}>最低价</Text>
                  {hasBaseline ? (
                    <Text style={{ color: TEXT_SUB, fontSize: 16, fontWeight: 600, lineHeight: '20px' }}>
                      ¥{sub.baseline_min_price!.toFixed(1)}
                    </Text>
                  ) : (
                    <Text style={{ color: TEXT_FAINT, fontSize: 14, lineHeight: '20px' }}>—</Text>
                  )}
                </div>

                {/* 分割线 */}
                <div style={{ width: 1, height: 32, background: 'rgba(255,255,255,0.06)' }} />

                {/* 通知次数 */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <Text style={{ color: TEXT_FAINT, fontSize: 11 }}>已通知</Text>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 3 }}>
                    <Text style={{
                      color: sub.notify_count > 0 ? ACCENT_GREEN : TEXT_FAINT,
                      fontSize: 16, fontWeight: 600, lineHeight: '20px',
                    }}>
                      {sub.notify_count}
                    </Text>
                    <Text style={{ color: TEXT_FAINT, fontSize: 11 }}>次</Text>
                  </div>
                </div>

                {/* 分割线 */}
                <div style={{ width: 1, height: 32, background: 'rgba(255,255,255,0.06)' }} />

                {/* 上次通知 */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <Text style={{ color: TEXT_FAINT, fontSize: 11 }}>上次通知</Text>
                  <Text style={{ color: TEXT_SUB, fontSize: 13, lineHeight: '20px' }}>
                    {fmtDate(sub.last_notify_at)}
                  </Text>
                </div>

                {/* 灵活填充 */}
                <div style={{ flex: 1 }} />

                {/* 通知开关 + 运行开关 */}
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <Tag
                    color={sub.notify_enabled ? 'blue' : 'default'}
                    style={{ margin: 0, borderRadius: 4, fontSize: 11, padding: '0 6px' }}
                  >
                    <MailOutlined style={{ marginRight: 3 }} />{sub.notify_enabled ? '通知开' : '通知关'}
                  </Tag>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Text style={{ color: TEXT_FAINT, fontSize: 12 }}>
                      {isActive ? '运行中' : '已停用'}
                    </Text>
                    <Switch
                      size="small"
                      checked={isActive}
                      onChange={() => handleToggle(sub)}
                    />
                  </div>
                </div>
              </div>

              {/* === 底栏：次要信息 === */}
              {(sub.cinema_address || sub.email) && (
                <div style={{
                  marginTop: 10, paddingTop: 10,
                  borderTop: '1px solid rgba(255,255,255,0.04)',
                  display: 'flex', gap: 16, flexWrap: 'wrap',
                }}>
                  {sub.cinema_address && (
                    <Text style={{ color: TEXT_FAINT, fontSize: 12 }}>
                      <EnvironmentOutlined style={{ marginRight: 4 }} />{sub.cinema_address}
                    </Text>
                  )}
                  {sub.email && (
                    <Text style={{ color: TEXT_FAINT, fontSize: 12 }}>
                      <MailOutlined style={{ marginRight: 4 }} />{sub.email}
                    </Text>
                  )}
                </div>
              )}
            </Card>
          )
        })}

        {/* 编辑抽屉 */}
        <SubscriptionDrawer
          open={drawerEditOpen}
          onClose={() => { setDrawerEditOpen(false); setEditingSub(null) }}
          editMode={true}
          editSubscription={editingSub}
          onSuccess={() => { setDrawerEditOpen(false); setEditingSub(null); loadSubs() }}
        />
      </div>
    </div>
  )
}
