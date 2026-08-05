// pages/PriceChangesPage.tsx — 票价变化页
// 筛选：影院 + 电影 + 时间段（默认过去7天）→ 点击查询按钮才加载数据
import { useState, useEffect, useMemo } from 'react'
import {
  Card, Select, Empty, Spin, Typography, Space, Button, message, Radio, DatePicker,
} from 'antd'
import { LineChartOutlined, SearchOutlined } from '@ant-design/icons'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import dayjs, { Dayjs } from 'dayjs'
import { useAuth } from '../store/auth'
import { getSubscribedCinemaMovies, getPriceChanges, type CinemaMovieItem } from '../api/endpoints'

const { Title, Text } = Typography
const { RangePicker } = DatePicker

interface PricePoint {
  time: string
  price_min: number
  price_avg: number
  price_max: number
}

type PriceType = 'min' | 'avg' | 'max'

export default function PriceChangesPage() {
  const { user } = useAuth()

  // 订阅的影院+电影组合列表
  const [cinemaMovies, setCinemaMovies] = useState<CinemaMovieItem[]>([])
  const [loadingList, setLoadingList] = useState(false)

  // 选中的筛选值
  const [selectedCinemaId, setSelectedCinemaId] = useState<number | undefined>()
  const [selectedMovieId, setSelectedMovieId] = useState<string | undefined>()

  // 时间范围（默认前7天到后7天）
  const [dateRange, setDateRange] = useState<[Dayjs, Dayjs]>([
    dayjs().subtract(7, 'day'),
    dayjs().add(7, 'day'),
  ])

  // 是否已点击查询
  const [queried, setQueried] = useState(false)

  // 价格类型
  const [priceType, setPriceType] = useState<PriceType>('min')

  // 票价走势数据
  const [priceData, setPriceData] = useState<PricePoint[] | null>(null)
  const [loadingPrices, setLoadingPrices] = useState(false)

  // 加载订阅的影院+电影组合
  useEffect(() => {
    if (!user) return
    setLoadingList(true)
    getSubscribedCinemaMovies()
      .then(res => {
        if (res.code === 0) {
          setCinemaMovies(res.data || [])
        }
      })
      .finally(() => setLoadingList(false))
  }, [user])

  // 影院列表（去重）
  const cinemaOptions = useMemo(() => {
    const seen = new Map<number, string>()
    for (const item of cinemaMovies) {
      if (!seen.has(item.cinema_id)) {
        seen.set(item.cinema_id, item.cinema_name)
      }
    }
    return Array.from(seen.entries()).map(([id, name]) => ({ id, name }))
  }, [cinemaMovies])

  // 选中影院后，该影院下的电影列表
  const movieOptions = useMemo(() => {
    if (!selectedCinemaId) return []
    return cinemaMovies
      .filter(item => item.cinema_id === selectedCinemaId)
      .map(item => ({
        movieId: item.movie_id,
        movieName: item.movie_name,
      }))
  }, [cinemaMovies, selectedCinemaId])

  // 切换影院时清空电影选择
  const onCinemaChange = (v: number) => {
    setSelectedCinemaId(v)
    setSelectedMovieId(undefined)
    setQueried(false)
    setPriceData(null)
  }

  const onMovieChange = (v: string) => {
    setSelectedMovieId(v)
    setQueried(false)
    setPriceData(null)
  }

  // 点击查询
  const handleQuery = () => {
    if (!selectedCinemaId || !selectedMovieId) {
      message.warning('请先选择影院和电影')
      return
    }
    if (!dateRange || dateRange.length !== 2) {
      message.warning('请选择时间段')
      return
    }
    setQueried(true)
    setLoadingPrices(true)
    const startDate = dateRange[0].format('YYYY-MM-DD')
    const endDate = dateRange[1].format('YYYY-MM-DD')
    getPriceChanges(selectedCinemaId, selectedMovieId, startDate, endDate)
      .then(res => {
        if (res.code === 0) {
          setPriceData(res.data?.trend || [])
        } else {
          setPriceData(null)
          message.error('加载票价变化数据失败')
        }
      })
      .finally(() => setLoadingPrices(false))
  }

  // 价格类型对应的 dataKey
  const priceDataKey = priceType === 'min' ? 'price_min' : priceType === 'avg' ? 'price_avg' : 'price_max'
  const priceLabel = priceType === 'min' ? '最低价' : priceType === 'avg' ? '平均价' : '最高价'
  const lineColor = priceType === 'min' ? '#e54847' : priceType === 'avg' ? '#faad14' : '#1677ff'

  if (!user) {
    return (
      <div style={{ maxWidth: 600, margin: '40px auto', padding: '0 16px' }}>
        <Card style={{ borderRadius: 12, background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <Empty description={<Text style={{ color: 'rgba(255,255,255,0.65)' }}>请先登录</Text>} />
        </Card>
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)', padding: '24px' }}>
      <div style={{ maxWidth: 1400, margin: '0 auto' }}>
        <Title level={3} style={{ color: '#fff', marginBottom: 24 }}>
          <LineChartOutlined style={{ marginRight: 8 }} />票价变化
        </Title>

        {/* 筛选区域 */}
        <Card
          size="small"
          style={{
            marginBottom: 24, borderRadius: 12,
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid rgba(255,255,255,0.08)',
          }}
        >
          <Space wrap size="middle" align="center">
            <Text style={{ color: 'rgba(255,255,255,0.65)', fontSize: 13 }}>筛选条件</Text>

            <Select
              showSearch
              value={selectedCinemaId}
              onChange={onCinemaChange}
              placeholder="选择影院"
              style={{ minWidth: 280 }}
              size="large"
              loading={loadingList}
              filterOption={(input, option) =>
                String(option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
              options={cinemaOptions.map(c => ({ label: c.name, value: c.id }))}
              notFoundContent={<Text style={{ color: '#999' }}>暂无订阅影院</Text>}
              allowClear
            />

            <Select
              showSearch
              value={selectedMovieId}
              onChange={onMovieChange}
              placeholder={selectedCinemaId ? '选择电影' : '请先选影院'}
              style={{ minWidth: 240 }}
              size="large"
              disabled={!selectedCinemaId}
              filterOption={(input, option) =>
                String(option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
              options={movieOptions.map(m => ({ label: m.movieName, value: m.movieId }))}
              notFoundContent={<Text style={{ color: '#999' }}>该影院暂无电影订阅</Text>}
              allowClear
            />

            <RangePicker
              value={dateRange}
              onChange={(dates) => {
                if (dates && dates.length === 2) {
                  setDateRange([dates[0]!, dates[1]!])
                } else {
                  setDateRange([dayjs().subtract(7, 'day'), dayjs()])
                }
                setQueried(false)
                setPriceData(null)
              }}
              size="large"
              allowClear={false}
              format="YYYY-MM-DD"
            />

            <Button
              type="primary"
              size="large"
              icon={<SearchOutlined />}
              onClick={handleQuery}
              disabled={!selectedCinemaId || !selectedMovieId}
            >
              查询
            </Button>
          </Space>
        </Card>

        {/* 票价走势图区域 */}
        {!queried ? (
          <Card
            style={{
              borderRadius: 12,
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.08)',
            }}
          >
            <Empty description={<Text style={{ color: '#999' }}>选择影院、电影和时间段后点击查询</Text>} />
          </Card>
        ) : loadingPrices ? (
          <div style={{ textAlign: 'center', padding: 80 }}>
            <Spin size="large"><Text style={{ color: '#fff' }}>加载票价数据...</Text></Spin>
          </div>
        ) : !priceData || priceData.length === 0 ? (
          <Card
            style={{
              borderRadius: 12,
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.08)',
            }}
          >
            <Empty description={<Text style={{ color: '#999' }}>所选时间范围内暂无票价变化数据</Text>} />
          </Card>
        ) : (
          <Card
            title={
              <Space>
                <LineChartOutlined style={{ color: '#1677ff' }} />
                <Text strong style={{ color: '#fff', fontSize: 16 }}>票价走势</Text>
              </Space>
            }
            extra={
              <Radio.Group
                value={priceType}
                onChange={(e) => setPriceType(e.target.value)}
                size="small"
                buttonStyle="solid"
              >
                <Radio.Button value="min">最低价</Radio.Button>
                <Radio.Button value="avg">平均价</Radio.Button>
                <Radio.Button value="max">最高价</Radio.Button>
              </Radio.Group>
            }
            style={{
              borderRadius: 12,
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.08)',
            }}
          >
            {priceData.length > 1 ? (
              <div style={{ width: '100%', height: 400 }}>
                <ResponsiveContainer>
                  <LineChart data={priceData}>
                    <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.08)" />
                    <XAxis dataKey="time" fontSize={11} stroke="rgba(255,255,255,0.45)" />
                    <YAxis fontSize={11} stroke="rgba(255,255,255,0.45)" />
                    <Tooltip
                      contentStyle={{ fontSize: 12, borderRadius: 8, background: '#1a1a2e', border: '1px solid rgba(255,255,255,0.12)' }}
                      formatter={(v: number) => [`¥${v}`, priceLabel]}
                    />
                    <Line type="monotone" dataKey={priceDataKey} stroke={lineColor} strokeWidth={2} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <Empty description="数据不足，至少需要 2 个数据点" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </Card>
        )}
      </div>
    </div>
  )
}
