// components/PinyinCityPicker.tsx — 拼音首字母索引城市选择器
//   仿手机通讯录风格: 左侧 A-Z 索引栏 + 右侧城市列表 + 顶部搜索
//   用法: <PinyinCityPicker cities={cities} value={cityId} onChange={setCityId} />
import { useState, useMemo, useRef, useCallback, useEffect } from 'react'
import { Input, Empty, Typography, Tag, Space } from 'antd'
import { SearchOutlined, EnvironmentOutlined } from '@ant-design/icons'

const { Text } = Typography

interface City { id: number; name: string; py: string }

interface Props {
  cities: City[]
  value?: number
  onChange?: (id: number) => void
  disabled?: boolean
}

type PinyinGroup = { letter: string; cities: City[] }

export default function PinyinCityPicker({ cities, value, onChange, disabled }: Props) {
  const [open, setOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [activeLetter, setActiveLetter] = useState('')
  const listRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // 关闭外部点击
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  // 拼音分组（A-Z + #）
  const groups = useMemo((): PinyinGroup[] => {
    const map: Record<string, City[]> = {}
    const filtered = keyword.trim()
      ? cities.filter(c => c.name.includes(keyword) || c.py.includes(keyword.toLowerCase()))
      : cities
    for (const c of filtered) {
      const l = c.py ? c.py[0].toUpperCase() : '#'
      if (!map[l]) map[l] = []
      map[l].push(c)
    }
    // 排序: A-Z then #
    return Object.keys(map).sort((a, b) => {
      if (a === '#') return 1
      if (b === '#') return -1
      return a.localeCompare(b)
    }).map(l => ({ letter: l, cities: map[l] }))
  }, [cities, keyword])

  const selectedName = useMemo(() => {
    if (!value) return ''
    const c = cities.find(c => c.id === value)
    return c ? c.name : ''
  }, [cities, value])

  // 点击字母 → 滚动到对应区域
  const scrollToLetter = useCallback((letter: string) => {
    setActiveLetter(letter)
    const el = listRef.current?.querySelector(`[data-letter="${letter}"]`)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }, [])

  // 拖动字母栏
  const [dragging, setDragging] = useState(false)
  const indexRef = useRef<HTMLDivElement>(null)

  const getLetterFromY = useCallback((clientY: number) => {
    const idxEl = indexRef.current
    if (!idxEl) return ''
    const children = idxEl.querySelectorAll('[data-letter]')
    for (const child of children) {
      const rect = child.getBoundingClientRect()
      if (clientY >= rect.top && clientY <= rect.bottom) {
        return child.getAttribute('data-letter') || ''
      }
    }
    return ''
  }, [])

  const handleIndexMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    setDragging(true)
    const letter = getLetterFromY(e.clientY)
    if (letter) scrollToLetter(letter)
  }, [getLetterFromY, scrollToLetter])

  const handleIndexMouseMove = useCallback((e: React.MouseEvent) => {
    if (!dragging) return
    const letter = getLetterFromY(e.clientY)
    if (letter) scrollToLetter(letter)
  }, [dragging, getLetterFromY, scrollToLetter])

  useEffect(() => {
    if (!dragging) return
    const handleUp = () => setDragging(false)
    document.addEventListener('mouseup', handleUp)
    return () => document.removeEventListener('mouseup', handleUp)
  }, [dragging])

  return (
    <div ref={containerRef} style={{ position: 'relative', display: 'inline-block' }}>
      {/* 触发器按钮 */}
      <div
        onClick={() => !disabled && setOpen(!open)}
        style={{
          display: 'flex', alignItems: 'center', gap: 8, padding: '6px 14px',
          background: 'rgba(255,255,255,0.06)', borderRadius: 8,
          border: '1px solid rgba(255,255,255,0.12)', cursor: disabled ? 'not-allowed' : 'pointer',
          transition: 'border-color 0.2s', minWidth: 200, height: 40,
          opacity: disabled ? 0.5 : 1,
          ...(open ? { borderColor: '#1677ff' } : {}),
        }}
      >
        <EnvironmentOutlined style={{ color: '#1677ff', fontSize: 16 }} />
        <Text style={{ color: selectedName ? '#fff' : 'rgba(255,255,255,0.45)', flex: 1, fontSize: 14 }}>
          {selectedName || '选择城市'}
        </Text>
        <Text style={{ color: 'rgba(255,255,255,0.25)', fontSize: 12 }}>▼</Text>
      </div>

      {/* 下拉面板 */}
      {open && (
        <div style={{
          position: 'absolute', top: 48, left: 0, zIndex: 1050,
          width: 520, maxHeight: 420,
          background: '#1a1a2e', borderRadius: 12,
          border: '1px solid rgba(255,255,255,0.12)',
          boxShadow: '0 8px 40px rgba(0,0,0,0.5)',
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}>
          {/* 搜索栏 */}
          <div style={{ padding: '12px 14px', borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
            <Input
              prefix={<SearchOutlined style={{ color: 'rgba(255,255,255,0.3)' }} />}
              placeholder="搜索城市名或拼音..."
              value={keyword}
              onChange={e => setKeyword(e.target.value)}
              allowClear
              autoFocus
              style={{ background: 'rgba(255,255,255,0.06)', border: 'none', color: '#fff', borderRadius: 8 }}
            />
          </div>

          {/* 城市列表 + 索引栏 */}
          <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
            {/* 城市列表 */}
            <div ref={listRef} style={{ flex: 1, overflowY: 'auto', padding: '4px 0' }}>
              {groups.length === 0 ? (
                <div style={{ padding: 40, textAlign: 'center' }}>
                  <Empty description={<Text style={{ color: '#999' }}>未匹配到城市</Text>} />
                </div>
              ) : (
                groups.map(g => (
                  <div key={g.letter} data-letter={g.letter}>
                    {/* 字母标题 */}
                    <div style={{
                      padding: '6px 14px', background: 'rgba(255,255,255,0.03)',
                      borderBottom: '1px solid rgba(255,255,255,0.04)',
                    }}>
                      <Text strong style={{ color: '#1677ff', fontSize: 13 }}>{g.letter}</Text>
                    </div>
                    {/* 城市列表 */}
                    <div style={{ display: 'flex', flexWrap: 'wrap', padding: '6px 10px', gap: 4 }}>
                      {g.cities.map(c => (
                        <Tag
                          key={c.id}
                          color={value === c.id ? 'blue' : undefined}
                          style={{
                            cursor: 'pointer', margin: 0, padding: '2px 10px',
                            borderRadius: 6, fontSize: 13,
                            transition: 'all 0.15s',
                            ...(value === c.id
                              ? { background: '#1677ff', color: '#fff', borderColor: '#1677ff' }
                              : { background: 'rgba(255,255,255,0.04)', color: 'rgba(255,255,255,0.75)', border: '1px solid rgba(255,255,255,0.06)' }
                            ),
                          }}
                          onClick={() => { onChange?.(c.id); setOpen(false) }}
                          onMouseEnter={e => {
                            if (value !== c.id) {
                              e.currentTarget.style.background = 'rgba(255,255,255,0.1)'
                              e.currentTarget.style.color = '#fff'
                            }
                          }}
                          onMouseLeave={e => {
                            if (value !== c.id) {
                              e.currentTarget.style.background = 'rgba(255,255,255,0.04)'
                              e.currentTarget.style.color = 'rgba(255,255,255,0.75)'
                            }
                          }}
                        >
                          {c.name}
                        </Tag>
                      ))}
                    </div>
                  </div>
                ))
              )}
            </div>

            {/* A-Z 拼音索引栏 */}
            <div
              ref={indexRef}
              onMouseDown={handleIndexMouseDown}
              onMouseMove={handleIndexMouseMove}
              style={{
                width: 30, display: 'flex', flexDirection: 'column',
                alignItems: 'center', justifyContent: 'center',
                padding: '8px 2px', background: 'rgba(255,255,255,0.02)',
                borderLeft: '1px solid rgba(255,255,255,0.06)',
                userSelect: 'none',
              }}
            >
              {groups.map(g => (
                <div
                  key={g.letter}
                  data-letter={g.letter}
                  style={{
                    fontSize: 11, fontWeight: 600, padding: '2px 4px', cursor: 'pointer',
                    color: activeLetter === g.letter ? '#1677ff' : 'rgba(255,255,255,0.45)',
                    transition: 'color 0.15s',
                  }}
                  onMouseEnter={() => scrollToLetter(g.letter)}
                >
                  {g.letter}
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
