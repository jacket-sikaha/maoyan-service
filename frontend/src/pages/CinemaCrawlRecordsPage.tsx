// pages/CinemaCrawlRecordsPage.tsx — 影院数据采集记录页（暗色仪表盘风格）
import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  Card, Table, Tag, Button, message, Typography, Space, Empty, Spin,
  DatePicker, Drawer, Radio, Row, Col, Breadcrumb,
} from 'antd'
import {
  ArrowLeftOutlined, HomeOutlined, DatabaseOutlined,
  VideoCameraOutlined, FieldTimeOutlined,
} from '@ant-design/icons'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip as RTooltip, ResponsiveContainer,
} from 'recharts'
import dayjs, { Dayjs } from 'dayjs'
import { useAuth } from '../store/auth'
import {
  getCrawlRecords, getSnapshotMovieShows,
  type CrawlRecordsDashboard, type MovieCrawlDetail,
} from '../api/endpoints'

const { Title, Text } = Typography
const { RangePicker } = DatePicker

// 暗色主题常量
const BG_PAGE = 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)'
const CARD_BG = 'rgba(255,255,255,0.04)'
const CARD_BORDER = '1px solid rgba(255,255,255,0.08)'
const TEXT_MAIN = '#fff'
const TEXT_SUB = 'rgba(255,255,255,0.65)'
const TEXT_FAINT = 'rgba(255,255,255,0.45)'
const ACCENT_RED = '#e54847'
const ACCENT_ORANGE = '#fa8c16'
const ACCENT_BLUE = '#1890ff'
const ACCENT_CYAN = '#13c2c2'
const ACCENT_GREEN = '#52c41a'

// 统计卡片
function StatCard({ icon, label, value, suffix, color }: {
  icon: React.ReactNode; label: string; value: number | string; suffix?: string; color?: string
}) {
  return (
    <Card size="small" style={{
      borderRadius: 10, background: CARD_BG, border: CARD_BORDER,
      backdropFilter: 'blur(8px)',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{
          width: 40, height: 40, borderRadius: 10, display: 'flex', alignItems: 'center', justifyContent: 'center',
          background: (color || ACCENT_BLUE) + '1a', fontSize: 20, color: color || ACCENT_BLUE,
        }}>
          {icon}
        </div>
        <div>
          <div style={{ color: TEXT_FAINT, fontSize: 12, marginBottom: 2 }}>{label}</div>
          <div style={{ color: color || TEXT_MAIN, fontSize: 22, fontWeight: 700, lineHeight: 1 }}>
            {value}{suffix && <span style={{ fontSize: 14, fontWeight: 400, marginLeft: 2 }}>{suffix}</span>}
          </div>
        </div>
      </div>
    </Card>
  )
}

interface ShowItem {
  movie_name: string
  cinema_name: string
  hall_name: string
  show_date: string
  show_time: string
  current_price: number
  vip_price?: number
  base_price?: number
  discount_price?: number
}

export default function CinemaCrawlRecordsPage() {
  const { id } = useParams<{ id: string }>()
  const { user } = useAuth()
  const navigate = useNavigate()

  const [dashboard, setDashboard] = useState<CrawlRecordsDashboard | null>(null)
  const [loading, setLoading] = useState(true)
  const [dateRange, setDateRange] = useState<[Dayjs, Dayjs]>([
    dayjs().subtract(7, 'day'),
    dayjs(),
  ])

  const [selectedMovieId, setSelectedMovieId] = useState<string>('')
  const [snapshotShows, setSnapshotShows] = useState<ShowItem[]>([])
  const [loadingShows, setLoadingShows] = useState(false)
  const [selectedSnapshotId, setSelectedSnapshotId] = useState<number | null>(null)

  const loadDashboard = useCallback(async () => {
    if (!id) return
    setLoading(true)
    try {
      const res = await getCrawlRecords(id, dateRange[0].format('YYYY-MM-DD'), dateRange[1].format('YYYY-MM-DD'))
      if (res.code === 0) {
        setDashboard(res.data)
        if (res.data.movies && res.data.movies.length > 0) {
          setSelectedMovieId(res.data.movies[0].movie_id)
        } else {
          setSelectedMovieId('')
        }
      } else {
        message.error('加载采集记录失败')
      }
    } catch { message.error('加载采集记录失败') }
    finally { setLoading(false) }
  }, [id, dateRange])

  useEffect(() => { if (user) loadDashboard() }, [user, loadDashboard])

  const handleRecordClick = async (snapshotId: number) => {
    if (!id || !selectedMovieId) return
    setSelectedSnapshotId(snapshotId)
    setLoadingShows(true)
    try {
      const res = await getSnapshotMovieShows(id, snapshotId, selectedMovieId)
      if (res.code === 0) {
        setSnapshotShows(res.data || [])
      } else {
        setSnapshotShows([])
      }
    } catch { setSnapshotShows([]) }
    finally { setLoadingShows(false) }
  }

  const handleMovieChange = (movieId: string) => {
    setSelectedMovieId(movieId)
    setSnapshotShows([])
    setSelectedSnapshotId(null)
  }

  const chartData = dashboard?.records?.slice().reverse().map(r => ({
    time: dayjs(r.fetched_at).format('MM-DD HH:mm'),
    showtimes: r.total_showtimes,
    movies: r.total_movies,
  })) || []

  // 暗色表格行样式
  const darkRowStyle = (record: any) => ({
    background: selectedSnapshotId === record.snapshot_id
      ? 'rgba(24,144,255,0.12)'
      : 'transparent',
  })

  // 采集记录表格列
  const recordColumns = [
    {
      title: '采集时间',
      dataIndex: 'fetched_at',
      key: 'fetched_at',
      width: 160,
      render: (v: string) => <Text style={{ color: TEXT_SUB, fontSize: 13 }}>{new Date(v).toLocaleString()}</Text>,
    },
    { title: '电影数', dataIndex: 'total_movies', key: 'movies', width: 70,
      render: (v: number) => <Text style={{ color: ACCENT_BLUE, fontWeight: 600 }}>{v}</Text>,
    },
    { title: '场次数', dataIndex: 'total_showtimes', key: 'showtimes', width: 70,
      render: (v: number) => <Text style={{ color: ACCENT_CYAN, fontWeight: 600 }}>{v}</Text>,
    },
    { title: '状态', dataIndex: 'parse_status', key: 'status', width: 70,
      render: (v: string) => v === 'success'
        ? <Tag color="green" style={{ margin: 0, fontSize: 11 }}>成功</Tag>
        : v === 'partial' ? <Tag color="orange" style={{ margin: 0, fontSize: 11 }}>部分</Tag>
          : <Tag color="red" style={{ margin: 0, fontSize: 11 }}>失败</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_: any, record: any) => (
        <Button
          type="link"
          size="small"
          onClick={() => handleRecordClick(record.snapshot_id)}
          style={{ padding: 0, color: selectedSnapshotId === record.snapshot_id ? ACCENT_BLUE : TEXT_SUB }}
        >
          查看明细
        </Button>
      ),
    },
  ]

  // 场次明细表格列
  const showColumns = [
    { title: '影厅', dataIndex: 'hall_name', key: 'hall', width: 80,
      render: (v: string) => <Text style={{ color: TEXT_SUB }}>{v}</Text>,
    },
    { title: '日期', dataIndex: 'show_date', key: 'date', width: 100,
      sorter: (a: ShowItem, b: ShowItem) => (a.show_date || '').localeCompare(b.show_date || ''),
      render: (v: string) => <Text style={{ color: TEXT_SUB }}>{v}</Text>,
    },
    { title: '时间', dataIndex: 'show_time', key: 'time', width: 70,
      sorter: (a: ShowItem, b: ShowItem) => (a.show_time || '').localeCompare(b.show_time || ''),
      render: (v: string) => <Text style={{ color: TEXT_SUB }}>{v}</Text>,
    },
    {
      title: '票价',
      dataIndex: 'current_price',
      key: 'price',
      width: 80,
      render: (v: number) => {
        const price = v ?? 0
        const allPrices = snapshotShows.map(s => s.current_price ?? 0).filter(p => p > 0)
        const isLowest = price > 0 && allPrices.length > 0 && price === Math.min(...allPrices)
        return (
          <Text strong style={{
            color: isLowest ? ACCENT_RED : TEXT_MAIN,
            fontSize: isLowest ? 15 : 13,
          }}>
            ¥{price.toFixed(1)}
          </Text>
        )
      },
      sorter: (a: ShowItem, b: ShowItem) => (a.current_price ?? 0) - (b.current_price ?? 0),
    },
    {
      title: '会员价', dataIndex: 'vip_price', key: 'vip', width: 75,
      render: (v?: number) => v && v > 0
        ? <Text style={{ color: TEXT_FAINT }}>¥{v.toFixed(1)}</Text>
        : <Text style={{ color: TEXT_FAINT }}>-</Text>,
    },
    {
      title: '基础价', dataIndex: 'base_price', key: 'base', width: 80,
      render: (v?: number) => v && v > 0
        ? <Text style={{ color: TEXT_FAINT }}>¥{v.toFixed(1)}</Text>
        : <Text style={{ color: TEXT_FAINT }}>-</Text>,
    },
    {
      title: '折扣价', dataIndex: 'discount_price', key: 'discount', width: 80,
      render: (v?: number) => v && v > 0
        ? <Tag color="orange" style={{ margin: 0, fontSize: 11 }}>¥{v.toFixed(1)}</Tag>
        : <Text style={{ color: TEXT_FAINT }}>-</Text>,
    },
  ]

  // ========== 渲染 ==========

  if (!user) return (
    <div style={{ minHeight: '100vh', background: BG_PAGE, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Card style={{ background: CARD_BG, border: CARD_BORDER, borderRadius: 12 }}>
        <Empty description={<Text style={{ color: TEXT_SUB }}>请先登录</Text>} />
      </Card>
    </div>
  )

  if (loading) return (
    <div style={{ minHeight: '100vh', background: BG_PAGE, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Spin size="large"><Text style={{ color: TEXT_SUB }}>加载中...</Text></Spin>
    </div>
  )

  if (!dashboard) return (
    <div style={{ minHeight: '100vh', background: BG_PAGE, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Card style={{ background: CARD_BG, border: CARD_BORDER, borderRadius: 12 }}>
        <Empty description={<Text style={{ color: TEXT_SUB }}>数据不存在</Text>} />
      </Card>
    </div>
  )

  const selectedMovie = dashboard.movies.find(x => x.movie_id === selectedMovieId)

  return (
    <div style={{ minHeight: '100vh', background: BG_PAGE, padding: '24px' }}>
      <div style={{ maxWidth: 1200, margin: '0 auto' }}>
        {/* 面包屑 */}
        <Breadcrumb
          items={[
            { title: <a href="/"><HomeOutlined /> <span style={{ color: TEXT_FAINT }}>首页</span></a> },
            { title: <a href="/subscriptions"><span style={{ color: TEXT_FAINT }}>订阅管理</span></a> },
            { title: <span style={{ color: TEXT_SUB }}>{dashboard.cinema_name || '影院'} 采集记录</span> },
          ]}
          style={{ marginBottom: 20 }}
        />

        {/* 页面标题 + 筛选 */}
        <Card
          size="small"
          style={{
            borderRadius: 10, marginBottom: 20,
            background: CARD_BG, border: CARD_BORDER,
            backdropFilter: 'blur(8px)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
            <Space>
              <Button
                icon={<ArrowLeftOutlined />}
                onClick={() => navigate('/subscriptions')}
                size="small"
                ghost
                style={{ borderRadius: 6, borderColor: 'rgba(255,255,255,0.15)', color: TEXT_SUB }}
              >
                返回
              </Button>
              <Title level={4} style={{ margin: 0, color: TEXT_MAIN }}>
                {dashboard.cinema_name || `影院#${dashboard.cinema_id}`}
                <Text style={{ color: TEXT_FAINT, fontSize: 14, fontWeight: 400, marginLeft: 8 }}>数据采集记录</Text>
              </Title>
            </Space>
            <RangePicker
              value={dateRange}
              onChange={(dates) => {
                if (dates && dates[0] && dates[1]) {
                  setDateRange([dates[0] as Dayjs, dates[1] as Dayjs])
                  setSnapshotShows([])
                  setSelectedSnapshotId(null)
                }
              }}
              allowClear={false}
              style={{ background: 'rgba(255,255,255,0.06)', borderColor: 'rgba(255,255,255,0.12)', borderRadius: 8 }}
            />
          </div>
        </Card>

        {/* 统计卡片行 */}
        <Row gutter={[16, 16]} style={{ marginBottom: 20 }}>
          <Col xs={12} sm={6}>
            <StatCard icon={<DatabaseOutlined />} label="采集次数" value={dashboard.total_snapshots} color={ACCENT_BLUE} />
          </Col>
          <Col xs={12} sm={6}>
            <StatCard icon={<FieldTimeOutlined />} label="总场次" value={dashboard.total_showtimes} color={ACCENT_CYAN} />
          </Col>
          <Col xs={12} sm={6}>
            <StatCard icon={<VideoCameraOutlined />} label="上映电影" value={dashboard.total_movies} color={ACCENT_GREEN} />
          </Col>
          <Col xs={12} sm={6}>
            <StatCard icon={<span style={{ fontSize: 16 }}>¥</span>} label="全局最低价" value={(dashboard.global_min_price || 0).toFixed(1)} color={ACCENT_RED} />
          </Col>
        </Row>

        {/* 采集趋势图 */}
        {chartData.length > 1 && (
          <Card
            size="small"
            style={{ borderRadius: 10, marginBottom: 20, background: CARD_BG, border: CARD_BORDER }}
          >
            <Text strong style={{ color: TEXT_MAIN, fontSize: 14, marginBottom: 8, display: 'block' }}>采集趋势</Text>
            <div style={{ width: '100%', height: 180 }}>
              <ResponsiveContainer>
                <AreaChart data={chartData}>
                  <defs>
                    <linearGradient id="colorShow" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor={ACCENT_BLUE} stopOpacity={0.4} />
                      <stop offset="95%" stopColor={ACCENT_BLUE} stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" />
                  <XAxis dataKey="time" fontSize={10} stroke="rgba(255,255,255,0.35)" />
                  <YAxis fontSize={10} stroke="rgba(255,255,255,0.35)" />
                  <RTooltip
                    contentStyle={{ fontSize: 12, borderRadius: 8, background: '#1a1a2e', border: '1px solid rgba(255,255,255,0.12)', color: '#fff' }}
                    labelStyle={{ color: 'rgba(255,255,255,0.6)' }}
                  />
                  <Area type="monotone" dataKey="showtimes" name="场次" stroke={ACCENT_BLUE} fill="url(#colorShow)" strokeWidth={2} />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          </Card>
        )}

        {/* 电影分组 Radio */}
        {dashboard.movies && dashboard.movies.length > 0 ? (
          <Card
            size="small"
            style={{ borderRadius: 10, marginBottom: 20, background: CARD_BG, border: CARD_BORDER }}
          >
            <Text strong style={{ color: TEXT_MAIN, fontSize: 14, marginBottom: 12, display: 'block' }}>
              电影列表 <Text style={{ color: TEXT_FAINT, fontSize: 12, fontWeight: 400 }}>按最低价排序</Text>
            </Text>

            <Radio.Group
              value={selectedMovieId}
              onChange={(e) => handleMovieChange(e.target.value)}
              style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}
            >
              {dashboard.movies.map((m: MovieCrawlDetail) => {
                const active = m.movie_id === selectedMovieId
                return (
                  <Radio.Button
                    key={m.movie_id}
                    value={m.movie_id}
                    style={{
                      borderRadius: 8,
                      background: active ? 'rgba(229,72,71,0.15)' : 'rgba(255,255,255,0.04)',
                      border: active ? `1px solid ${ACCENT_RED}66` : '1px solid rgba(255,255,255,0.08)',
                      color: active ? ACCENT_RED : TEXT_SUB,
                      padding: '4px 12px',
                      height: 'auto',
                      lineHeight: '20px',
                    }}
                  >
                    <Space size={6}>
                      <span style={{ fontSize: 13, fontWeight: active ? 600 : 400 }}>{m.movie_name}</span>
                      <span style={{
                        fontSize: 11, fontWeight: 700,
                        color: active ? ACCENT_RED : ACCENT_RED + 'cc',
                      }}>¥{m.min_price.toFixed(1)}</span>
                      <span style={{ fontSize: 11, color: TEXT_FAINT }}>{m.showtimes}场</span>
                    </Space>
                  </Radio.Button>
                )
              })}
            </Radio.Group>

            {/* 选中电影的价格统计 */}
            {selectedMovie && (
              <Row gutter={24} style={{ marginTop: 16 }}>
                <Col span={8}>
                  <div style={{ textAlign: 'center', padding: '8px 0' }}>
                    <div style={{ color: TEXT_FAINT, fontSize: 12, marginBottom: 4 }}>最低价</div>
                    <div style={{ color: ACCENT_RED, fontSize: 24, fontWeight: 700, lineHeight: 1 }}>
                      ¥{selectedMovie.min_price.toFixed(1)}
                    </div>
                  </div>
                </Col>
                <Col span={8}>
                  <div style={{ textAlign: 'center', padding: '8px 0', borderLeft: '1px solid rgba(255,255,255,0.06)', borderRight: '1px solid rgba(255,255,255,0.06)' }}>
                    <div style={{ color: TEXT_FAINT, fontSize: 12, marginBottom: 4 }}>均价</div>
                    <div style={{ color: TEXT_MAIN, fontSize: 24, fontWeight: 700, lineHeight: 1 }}>
                      ¥{selectedMovie.avg_price.toFixed(1)}
                    </div>
                  </div>
                </Col>
                <Col span={8}>
                  <div style={{ textAlign: 'center', padding: '8px 0' }}>
                    <div style={{ color: TEXT_FAINT, fontSize: 12, marginBottom: 4 }}>最高价</div>
                    <div style={{ color: ACCENT_ORANGE, fontSize: 24, fontWeight: 700, lineHeight: 1 }}>
                      ¥{selectedMovie.max_price.toFixed(1)}
                    </div>
                  </div>
                </Col>
              </Row>
            )}
          </Card>
        ) : null}

        {/* 采集记录表格 */}
        <Card
          size="small"
          style={{ borderRadius: 10, marginBottom: 20, background: CARD_BG, border: CARD_BORDER }}
        >
          <Text strong style={{ color: TEXT_MAIN, fontSize: 14, marginBottom: 12, display: 'block' }}>采集记录</Text>
          {dashboard.records && dashboard.records.length > 0 ? (
            <Table
              columns={recordColumns}
              dataSource={dashboard.records}
              rowKey="snapshot_id"
              pagination={{ pageSize: 8, size: 'small' }}
              size="small"
              scroll={{ x: 500 }}
              rowClassName={(record) => selectedSnapshotId === record.snapshot_id ? 'ant-table-row-selected' : ''}
              onRow={(record) => ({
                onClick: () => handleRecordClick(record.snapshot_id),
                style: { cursor: 'pointer', ...darkRowStyle(record) },
              })}
            />
          ) : (
            <Empty description={<Text style={{ color: TEXT_FAINT }}>该时间段内暂无采集记录</Text>} />
          )}
        </Card>

        {/* 场次明细抽屉 */}
        <Drawer
          title={
            <Space>
              <Text strong style={{ color: TEXT_MAIN, fontSize: 15 }}>场次明细</Text>
              {selectedMovie && (
                <Tag color="blue" style={{ margin: 0, fontSize: 12 }}>《{selectedMovie.movie_name}》</Tag>
              )}
              {(() => {
                const r = dashboard.records.find(x => x.snapshot_id === selectedSnapshotId)
                return r ? (
                  <Text style={{ color: TEXT_FAINT, fontSize: 12 }}>
                    采集于 {new Date(r.fetched_at).toLocaleString()}
                  </Text>
                ) : null
              })()}
            </Space>
          }
          open={!!selectedSnapshotId}
          onClose={() => setSelectedSnapshotId(null)}
          width={560}
          styles={{
            header: { background: '#0f0f1a', borderBottom: '1px solid rgba(255,255,255,0.08)' },
            body: { background: '#0f0f1a', padding: '16px 20px' },
          }}
        >
          {loadingShows ? (
            <div style={{ textAlign: 'center', padding: 60 }}>
              <Spin tip="加载中..." />
            </div>
          ) : snapshotShows.length > 0 ? (
            <Table
              columns={showColumns}
              dataSource={snapshotShows}
              rowKey={(_, i) => String(i)}
              pagination={false}
              size="small"
              scroll={{ x: 520 }}
            />
          ) : (
            <Empty description={<Text style={{ color: TEXT_FAINT }}>该快照中无此电影的场次数据</Text>} />
          )}
        </Drawer>
      </div>
    </div>
  )
}
