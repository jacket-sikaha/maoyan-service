// pages/HomePage.tsx — 首页重设计：Grid 电影卡片 + 猫眼完整字段展示
//   设计风格：大卡片 grid 2-3列布局，海报蒙版+渐变叠加层，信息层叠式排版
//   交互：点击电影卡片 → 跳转 /movie/:id?name=电影名 票价查询页
import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Tag, Typography, Space, Row, Col, Empty, Spin, Flex } from 'antd'
import {
  StarFilled, EyeOutlined, CalendarOutlined, RightOutlined,
} from '@ant-design/icons'
import { getHotMovies } from '../api/endpoints'

const { Title, Text } = Typography

interface Movie {
  movie_id: number; name: string; img: string; score: number;
  version: string; star: string; release_date: string;
  show_info: string; showst: number; wish: number;
  global_released: boolean; coming_title: string;
}

export default function HomePage() {
  const navigate = useNavigate()
  const [movies, setMovies] = useState<Movie[]>([])
  const [loadingMovies, setLoadingMovies] = useState(false)

  useEffect(() => {
    setLoadingMovies(true)
    getHotMovies(1).then(res => {
      if (res.code === 0) setMovies(res.data)
    }).finally(() => setLoadingMovies(false))
  }, [])

  const handleClickMovie = (m: Movie) => {
    navigate(`/movie/${m.movie_id}?name=${encodeURIComponent(m.name)}`)
  }

  // 卡片颜色方案
  const cardColors = ['#2d3436', '#6c5ce7', '#0984e3', '#00b894', '#e17055', '#fdcb6e', '#636e72', '#d63031', '#a29bfe', '#fd79a8']

  return (
    <div style={{ minHeight: '100vh', background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)' }}>
      {/* 顶部 Hero 区域 */}
      <div style={{
        padding: '48px 24px 40px', textAlign: 'center',
        background: 'linear-gradient(160deg, #1a1a2e 0%, #16213e 50%, #0a0a0a 100%)',
        borderBottom: '1px solid rgba(255,255,255,0.06)',
      }}>
        <Title level={1} style={{
          color: '#fff', fontSize: 42, fontWeight: 800, marginBottom: 8,
          background: 'linear-gradient(135deg, #f5f5f5 0%, #a8a8a8 100%)',
          WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent',
        }}>
          热映电影
        </Title>
        <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 15 }}>
          实时猫眼票价监控 · 点击卡片查看排片票价
        </Text>
      </div>

      {/* 电影卡片 Grid */}
      <div style={{ maxWidth: 1400, margin: '0 auto', padding: '32px 24px' }}>
        {loadingMovies ? (
          <div style={{ textAlign: 'center', padding: 80 }}>
            <Spin size="large"><Text style={{ color: '#fff' }}>加载热映电影...</Text></Spin>
          </div>
        ) : movies.length === 0 ? (
          <Empty description={<Text style={{ color: '#999' }}>暂无热映电影</Text>} />
        ) : (
          <Row gutter={[20, 20]}>
            {movies.map((m, i) => {
              const isComing = m.showst === 4
              const colorAccent = cardColors[i % cardColors.length]
              return (
                <Col key={m.movie_id} xs={24} sm={12} lg={8}>
                  <div
                    onClick={() => handleClickMovie(m)}
                    style={{
                      position: 'relative', borderRadius: 16, overflow: 'hidden',
                      height: 380, cursor: 'pointer',
                      background: `linear-gradient(180deg, ${colorAccent}22 0%, ${colorAccent}66 100%)`,
                      transition: 'transform 0.3s, box-shadow 0.3s',
                      boxShadow: '0 4px 24px rgba(0,0,0,0.3)',
                    }}
                    onMouseEnter={e => {
                      e.currentTarget.style.transform = 'translateY(-6px)'
                      e.currentTarget.style.boxShadow = `0 12px 40px ${colorAccent}44`
                    }}
                    onMouseLeave={e => {
                      e.currentTarget.style.transform = 'translateY(0)'
                      e.currentTarget.style.boxShadow = '0 4px 24px rgba(0,0,0,0.3)'
                    }}
                  >
                    {/* 海报背景 */}
                    {m.img && (
                      <img
                        src={m.img}
                        alt={m.name}
                        style={{
                          position: 'absolute', inset: 0, width: '100%', height: '100%',
                          objectFit: 'cover', opacity: 0.5, filter: 'blur(1px)',
                        }}
                      />
                    )}

                    {/* 渐变叠加 */}
                    <div style={{
                      position: 'absolute', inset: 0,
                      background: `linear-gradient(180deg, rgba(0,0,0,0.1) 0%, rgba(0,0,0,0.7) 70%, rgba(0,0,0,0.9) 100%)`,
                    }} />

                    {/* 内容 */}
                    <div style={{ position: 'relative', height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'flex-end', padding: 20 }}>
                      {/* 评分徽章 */}
                      <div style={{ position: 'absolute', top: 16, right: 16 }}>
                        {m.score > 0 ? (
                          <div style={{
                            background: 'rgba(245, 158, 11, 0.9)',
                            color: '#fff', borderRadius: 8, padding: '4px 12px',
                            fontSize: 20, fontWeight: 800, lineHeight: 1.2,
                            display: 'flex', alignItems: 'center', gap: 4,
                          }}>
                            <StarFilled style={{ fontSize: 14 }} />
                            {m.score.toFixed(1)}
                          </div>
                        ) : (
                          <Tag color={isComing ? 'blue' : 'default'} style={{ borderRadius: 8, fontSize: 13 }}>
                            {isComing ? '即将上映' : '暂无评分'}
                          </Tag>
                        )}
                      </div>

                      {/* 电影标题 */}
                      <Title level={2} style={{
                        color: '#fff', fontSize: 28, fontWeight: 800, marginBottom: 2,
                        textShadow: '0 2px 8px rgba(0,0,0,0.5)',
                        lineHeight: 1.2,
                      }}>
                        {m.name}
                      </Title>

                      {/* 副标题信息栏 */}
                      <Space size={4} style={{ marginBottom: 8 }} wrap>
                        {m.version && (
                          <Tag color="purple" style={{ borderRadius: 6, fontSize: 11 }}>{m.version.toUpperCase()}</Tag>
                        )}
                        {m.wish > 0 && (
                          <Text style={{ color: 'rgba(255,255,255,0.6)', fontSize: 12 }}>
                            <EyeOutlined style={{ marginRight: 3 }} />
                            {m.wish > 10000 ? `${(m.wish / 10000).toFixed(1)}万` : m.wish} 想看
                          </Text>
                        )}
                      </Space>

                      {/* 主演 */}
                      {m.star && (
                        <Text style={{
                          color: 'rgba(255,255,255,0.5)', fontSize: 13,
                          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                          marginBottom: 4,
                        }}>
                          主演：{m.star}
                        </Text>
                      )}

                      {/* 上映信息 */}
                      <Flex justify="space-between" align="center" style={{ marginTop: 8 }}>
                        <Space size={4}>
                          <CalendarOutlined style={{ color: 'rgba(255,255,255,0.5)', fontSize: 12 }} />
                          <Text style={{ color: 'rgba(255,255,255,0.5)', fontSize: 12 }}>
                            {m.release_date ? `${m.release_date} 上映` : ''}
                          </Text>
                        </Space>
                        {m.show_info && (
                          <Text style={{ color: 'rgba(255,255,255,0.5)', fontSize: 12 }}>
                            {m.show_info}
                          </Text>
                        )}
                      </Flex>

                      {/* 底部操作栏 */}
                      <div style={{
                        marginTop: 12, paddingTop: 12,
                        borderTop: '1px solid rgba(255,255,255,0.1)',
                        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                      }}>
                        <Text style={{ color: 'rgba(255,255,255,0.5)', fontSize: 13 }}>
                          <RightOutlined /> 查看排片票价
                        </Text>
                        <Tag
                          color={isComing ? 'processing' : 'success'}
                          style={{ borderRadius: 8, fontSize: 11 }}
                        >
                          {isComing ? m.coming_title || '即将上映' : '热映中'}
                        </Tag>
                      </div>
                    </div>
                  </div>
                </Col>
              )
            })}
          </Row>
        )}
      </div>
    </div>
  )
}
