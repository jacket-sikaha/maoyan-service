// pages/MoviePricePage.tsx — 热映电影票价查询页面
//   上半部：城市 → 区/县树形选择 筛选表单
//   下半部：按影院分组的票价表格（多列可排序） + CSV 导出（需登录）
import { useState, useEffect, useMemo, useCallback } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import {
  Card, Table, Tag, TreeSelect, Button, message, Typography, Space, Row, Col,
  Empty, Spin, Breadcrumb, Tooltip, Flex,
} from 'antd'
import {
  DownloadOutlined, EnvironmentOutlined, SearchOutlined,
  HomeOutlined, FilterOutlined, BellOutlined
} from '@ant-design/icons'
import { getCities, getDistricts, queryShows, exportShowsCSV } from '../api/endpoints'
import { useAuth } from '../store/auth'
import PinyinCityPicker from '../components/PinyinCityPicker'
import SubscriptionDrawer from '../components/SubscriptionDrawer'

const { Title, Text } = Typography

interface Movie {
  movie_id: number; name: string; img: string; score: number
  version: string; star: string; release_date: string
  show_info: string; showst: number; wish: number
  global_released: boolean; coming_title: string
}
interface City { id: number; name: string; py: string }
interface DistrictNode {
  id: number; name: string; cinema_count: number;
  sub_items?: DistrictNode[]
}
interface ShowItem {
  cinema_id: number
  cinema_name: string; cinema_address: string; distance_km: number
  show_date: string; show_time: string; end_time: string
  hall_name: string; language: string; price: number
  vip_price: number; base_price: number; discount_price: number
}

export default function MoviePricePage() {
  const { id: movieId } = useParams()
  const [searchParams] = useSearchParams()
  const { user } = useAuth()

  const movieName = searchParams.get('name') || ''

  // 筛选状态
  const [cities, setCities] = useState<City[]>([])
  const [cityId, setCityId] = useState<number | undefined>()
  const [districts, setDistricts] = useState<DistrictNode[]>([])
  const [districtId, setDistrictId] = useState<number | undefined>()
  const [areaId, setAreaId] = useState<number | undefined>()
  const [loadingDistricts, setLoadingDistricts] = useState(false)

  // 查询状态
  const [shows, setShows] = useState<ShowItem[]>([])
  const [loadingShows, setLoadingShows] = useState(false)
  const [queried, setQueried] = useState(false)

  // 导出
  const [exporting, setExporting] = useState(false)

  // 订阅抽屉
  const [drawerOpen, setDrawerOpen] = useState(false)

  // 每影院均价+cinema_id，传给 SubscriptionDrawer
  const cinemaAvgPrices = useMemo(() => {
    const grouped: Record<string, { total: number; count: number; cinema_id: number }> = {}
    shows.forEach(s => {
      if (!grouped[s.cinema_name]) grouped[s.cinema_name] = { total: 0, count: 0, cinema_id: s.cinema_id || 0 }
      grouped[s.cinema_name].total += s.price
      grouped[s.cinema_name].count += 1
      if (!grouped[s.cinema_name].cinema_id) grouped[s.cinema_name].cinema_id = s.cinema_id || 0
    })
    return Object.entries(grouped).map(([name, v]) => ({
      cinema_name: name,
      cinema_id: v.cinema_id,
      price: v.count > 0 ? v.total / v.count : 0,
    }))
  }, [shows])

  // 加载城市
  useEffect(() => {
    getCities().then(res => { if (res.code === 0) setCities(res.data) })
  }, [])

  // 城市变化 → 拉区县
  useEffect(() => {
    if (!cityId) { setDistricts([]); setDistrictId(undefined); return }
    setLoadingDistricts(true)
    getDistricts(cityId).then(res => {
      if (res.code === 0) setDistricts(res.data)
      else setDistricts([])
    }).finally(() => setLoadingDistricts(false))
  }, [cityId])

  // 区县选项（树形，带影院数）
  const districtTreeOptions = useMemo(() => {
    return districts.map(d => {
      // 选某个区/县（不是具体片区）
      const mainNode: any = {
        title: `${d.name}（${d.cinema_count}家）`,
        value: d.id,
        key: `d-${d.id}`,
      }
      if (d.sub_items && d.sub_items.length > 0) {
        mainNode.children = d.sub_items.map(sub => ({
          title: `${sub.name}（${sub.cinema_count}家）`,
          value: sub.id,
          key: `a-${sub.id}`,
        }))
      }
      return mainNode
    })
  }, [districts])

  // 城市列表（给 PinyinCityPicker 用）
  const cityList = useMemo(() => cities.map(c => ({ id: c.id, name: c.name, py: c.py })), [cities])

  // 解析树选择（区分区县ID vs 片区ID）
  const resolveArea = useCallback((selId: number | undefined) => {
    if (!selId) return { district_id: undefined, area_id: undefined }
    // 检查是否在顶层 district 中
    for (const d of districts) {
      if (d.id === selId) return { district_id: d.id, area_id: undefined }
      if (d.sub_items) {
        for (const sub of d.sub_items) {
          if (sub.id === selId) return { district_id: d.id, area_id: sub.id }
        }
      }
    }
    return { district_id: selId, area_id: undefined }
  }, [districts])

  // 查询票价
  const handleQuery = async () => {
    if (!cityId || !movieId) { message.warning('请先选择城市'); return }
    setLoadingShows(true); setQueried(false)
    try {
      const { district_id, area_id } = resolveArea(districtId)
      const res = await queryShows({
        city_id: cityId,
        district_id: district_id ?? -1,
        area_id: area_id ?? -1,
        movie_id: Number(movieId),
        max: 50,
      })
      if (res.code === 0) {
        setShows(res.data.shows || [])
        setQueried(true)
      }
    } finally { setLoadingShows(false) }
  }

  // CSV 导出（需登录）
  const handleExport = async () => {
    if (!user) { message.warning('请先登录后再导出'); return }
    if (!cityId || !movieId) return
    setExporting(true)
    try {
      const { district_id, area_id } = resolveArea(districtId)
      const res: any = await exportShowsCSV({
        city_id: cityId, movie_id: Number(movieId),
        district_id: district_id ?? -1, area_id: area_id ?? -1, max: 100,
      })
      const url = window.URL.createObjectURL(new Blob([res]))
      const a = document.createElement('a'); a.href = url
      a.download = `${movieName || 'movie'}_票价_${new Date().toISOString().slice(0, 10)}.csv`
      a.click(); window.URL.revokeObjectURL(url)
      message.success('CSV 导出成功')
    } catch { message.error('导出失败') }
    finally { setExporting(false) }
  }

  // 按影院分组
  const cinemaGroups = useMemo(() => {
    const g: Record<string, ShowItem[]> = {}
    for (const s of shows) {
      if (!g[s.cinema_name]) g[s.cinema_name] = []
      g[s.cinema_name].push(s)
    }
    return g
  }, [shows])

  // 表格列
  const showColumns: any[] = [
    { title: '影厅', dataIndex: 'hall_name', width: 75 },
    {
      title: '日期', dataIndex: 'show_date', width: 95,
      sorter: (a: ShowItem, b: ShowItem) => a.show_date.localeCompare(b.show_date),
    },
    {
      title: '开场', dataIndex: 'show_time', width: 75,
      sorter: (a: ShowItem, b: ShowItem) => (a.show_time || '').localeCompare(b.show_time || ''),
    },
    { title: '散场', dataIndex: 'end_time', width: 70 },
    { title: '版本', dataIndex: 'language', width: 55 },
    {
      title: '票价', dataIndex: 'price', width: 80,
      render: (v: number, r: ShowItem) => {
        const isLowest = r.price > 0 && r.price === Math.min(...shows.map(s => s.price).filter(p => p > 0))
        return <Text strong style={{ color: isLowest ? '#e54847' : '#fff', fontSize: 13 }}>
          ¥{v.toFixed(1)}
        </Text>
      },
      sorter: (a: ShowItem, b: ShowItem) => a.price - b.price,
    },
    {
      title: '优惠价', dataIndex: 'discount_price', width: 85,
      render: (v: number) => v > 0
        ? <Tag color="orange" style={{ margin: 0, fontSize: 12 }}>¥{v.toFixed(1)}</Tag>
        : '-',
      sorter: (a: ShowItem, b: ShowItem) => a.discount_price - b.discount_price,
    },
    {
      title: '原价', dataIndex: 'base_price', width: 80,
      render: (v: number) => v > 0 ? <Text type="secondary" style={{ fontSize: 12 }}>¥{v.toFixed(1)}</Text> : '-',
      sorter: (a: ShowItem, b: ShowItem) => a.base_price - b.base_price,
    },
    {
      title: '会员', dataIndex: 'vip_price', width: 75,
      render: (v: number) => v > 0 ? <Text type="secondary" style={{ fontSize: 12 }}>¥{v.toFixed(1)}</Text> : '-'
    },
  ]

  // 统计
  const priceStats = useMemo(() => {
    const valid = shows.filter(s => s.price > 0)
    if (!valid.length) return null
    const prices = valid.map(s => s.price)
    return {
      count: valid.length,
      min: Math.min(...prices),
      max: Math.max(...prices),
      avg: prices.reduce((a, b) => a + b, 0) / prices.length,
    }
  }, [shows])

  return (
    <div style={{ minHeight: '100vh', background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)', padding: '24px' }}>
      <div style={{ maxWidth: 1400, margin: '0 auto' }}>
        {/* 面包屑 */}
        <Breadcrumb
          items={[
            { title: <a href="/"><HomeOutlined /> 首页</a> },
            { title: movieName || `电影 #${movieId}` },
          ]}
          style={{ marginBottom: 20 }}
        />

        {/* 电影信息标题 */}
        <Flex align="center" gap={16} style={{ marginBottom: 24 }}>
          <Title level={2} style={{ color: '#fff', margin: 0, fontWeight: 800 }}>{movieName}</Title>
          {movieId && (
            <Text type="secondary" style={{ fontSize: 14 }}>猫眼ID: {movieId}</Text>
          )}
        </Flex>

        {/* ======== 筛选表单 ======== */}
        <Card
          size="small"
          style={{
            marginBottom: 24, borderRadius: 12,
            background: 'rgba(255,255,255,0.04)',
            backdropFilter: 'blur(20px)',
            border: '1px solid rgba(255,255,255,0.08)',
          }}
        >
          <Flex align="center" gap={16} wrap="wrap">
            <Space align="center">
              <FilterOutlined style={{ color: '#1677ff' }} />
              <Text style={{ color: 'rgba(255,255,255,0.65)', fontSize: 13 }}>筛选条件</Text>
            </Space>

            {/* 城市选择（拼音索引 + 搜索） */}
            <PinyinCityPicker
              cities={cityList}
              value={cityId}
              onChange={(v: number) => { setCityId(v); setDistrictId(undefined) }}
            />

            {/* 区/县树形选择 */}
            <TreeSelect
              showSearch
              value={districtId}
              onChange={(v: any) => setDistrictId(v)}
              treeData={districtTreeOptions}
              placeholder={cityId ? '选择区/县（可选，不选则查全部）' : '请先选城市'}
              style={{ minWidth: 260 }}
              size="large"
              disabled={!cityId}
              loading={loadingDistricts}
              treeDefaultExpandAll
              filterTreeNode={(input, node) =>
                String(node?.title ?? '').toLowerCase().includes(input.toLowerCase())
              }
              allowClear
            />

            {/* 查询按钮 */}
            <Button
              type="primary" size="large" icon={<SearchOutlined />}
              onClick={handleQuery} loading={loadingShows}
              disabled={!cityId}
              style={{ borderRadius: 8 }}
            >
              查询票价
            </Button>

            {/* CSV 导出按钮（需登录） */}
            <Tooltip title={user ? '导出 CSV' : '请先登录后再导出'}>
              <Button
                size="large" icon={<DownloadOutlined />}
                onClick={handleExport} loading={exporting}
                disabled={!queried || shows.length === 0}
                style={{ borderRadius: 8 }}
              >
                导出 CSV
              </Button>
            </Tooltip>

            {/* 订阅价格按钮 */}
            <Tooltip title={!user ? '请先登录' : !queried || shows.length === 0 ? '请先查询票价' : '订阅电影票价'}>
              <Button
                size="large"
                icon={<BellOutlined />}
                onClick={() => setDrawerOpen(true)}
                disabled={!user || !queried || shows.length === 0}
                style={{ borderRadius: 8 }}
              >
                订阅价格
              </Button>
            </Tooltip>
          </Flex>
        </Card>

        {/* 价格统计摘要 */}
        {priceStats && queried && (
          <Card
            size="small"
            style={{
              marginBottom: 16, borderRadius: 12,
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.08)',
            }}
          >
            <Row gutter={16}>
              <Col span={6}><Text type="secondary">总场次</Text><br /><Text strong style={{ color: '#fff', fontSize: 18 }}>{priceStats.count}</Text></Col>
              <Col span={6}><Text type="secondary">最低价</Text><br /><Text strong style={{ color: '#52c41a', fontSize: 18 }}>¥{priceStats.min.toFixed(1)}</Text></Col>
              <Col span={6}><Text type="secondary">最高价</Text><br /><Text strong style={{ color: '#ff4d4f', fontSize: 18 }}>¥{priceStats.max.toFixed(1)}</Text></Col>
              <Col span={6}><Text type="secondary">均价</Text><br /><Text strong style={{ color: '#fff', fontSize: 18 }}>¥{priceStats.avg.toFixed(1)}</Text></Col>
            </Row>
          </Card>
        )}

        {/* ======== 排片表格 ======== */}
        {loadingShows ? (
          <div style={{ textAlign: 'center', padding: 80 }}>
            <Spin size="large"><Text style={{ color: '#fff' }}>查询排片票价中...</Text></Spin>
          </div>
        ) : !queried ? (
          <Empty
            description={<Text style={{ color: '#999' }}>请选择城市和区/县后点击「查询票价」</Text>}
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          />
        ) : shows.length === 0 ? (
          <Empty description={<Text style={{ color: '#999' }}>暂未查到排片数据，试试换一个区域</Text>} />
        ) : (
          <>
            <Text type="secondary" style={{ marginBottom: 12, display: 'block' }}>
              共 {Object.keys(cinemaGroups).length} 家影院 · {shows.length} 场排片
            </Text>

            {Object.entries(cinemaGroups).map(([cinema, cinemaShows]) => (
              <Card
                key={cinema}
                size="small"
                title={
                  <Flex justify="space-between" align="center">
                    <Space>
                      <Text strong style={{ fontSize: 15 }}>{cinema}</Text>
                      <Tag icon={<EnvironmentOutlined />} color="blue">
                        {cinemaShows[0].distance_km.toFixed(1)}km
                      </Tag>
                      <Text type="secondary" style={{ fontSize: 12 }}>{cinemaShows[0].cinema_address.slice(0, 18)}...</Text>
                    </Space>
                  </Flex>
                }
                style={{
                  marginBottom: 12, borderRadius: 10,
                  background: 'rgba(255,255,255,0.03)',
                  border: '1px solid rgba(255,255,255,0.06)',
                }}
              >
                <Table
                  columns={showColumns}
                  dataSource={cinemaShows}
                  rowKey={(_, i) => String(i)}
                  pagination={false}
                  size="small"
                  showSorterTooltip={false}
                />
              </Card>
            ))}
          </>
        )}

        {/* 订阅抽屉 */}
        <SubscriptionDrawer
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          cityId={cityId}
          cityName={cities.find(c => c.id === cityId)?.name || ''}
          movieId={Number(movieId)}
          movieName={movieName}
          cinemas={cinemaAvgPrices}
          onSuccess={() => { setDrawerOpen(false); }}
        />
      </div>
    </div>
  )
}
