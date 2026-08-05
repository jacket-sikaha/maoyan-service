// components/SubscriptionDrawer.tsx — 订阅创建/编辑抽屉（从右侧滑出）
// 支持影院级 + 电影级订阅：
//   - 影院级：MovieID 为空，监控影院所有排片最低价
//   - 电影级：指定 MovieID，只监控该电影最低价
//
// 创建模式：
//   1. 影院选择改为下拉选择框（Select），不再手输
//   2. 通知邮箱默认当前登录用户邮箱，只读不可改
//   3. 目标票价必须先选影院，换影院清空重置；填写价格不能高于所选影院平均票价
//
// 编辑模式：
//   1. 可修改目标票价（只能调低）、备注、启用/禁用
//   2. 可删除当前订阅
import { useEffect, useState, useMemo } from 'react'
import {
  Drawer, Form, Input, InputNumber, Button, message, Modal,
  Typography, Space, Divider, Select, Switch, Popconfirm,
} from 'antd'
import { createSubscription, updateSubscription, deleteSubscription } from '../api/endpoints'
import { useAuth } from '../store/auth'

const { Text } = Typography

interface CinemaProp {
  cinema_name: string
  cinema_id?: number
  price: number  // 该影院排片票价平均数
}

interface SubscriptionDrawerProps {
  cityId?: number
  cityName?: string
  movieId?: number
  movieName?: string
  cinemas?: CinemaProp[]
  // 编辑模式
  editMode?: boolean
  editSubscription?: any
  open: boolean
  onClose: () => void
  onSuccess?: () => void
}

export default function SubscriptionDrawer(props: SubscriptionDrawerProps) {
  const {
    cityId, cityName, movieId, movieName,
    cinemas = [],
    editMode = false, editSubscription,
    open, onClose, onSuccess,
  } = props

  const { user } = useAuth()
  const [form] = Form.useForm()

  // 当前选中的影院（用于创建模式校验目标票价上限）
  const [selectedCinema, setSelectedCinema] = useState<CinemaProp | null>(null)
  // 编辑模式：启用/禁用状态（独立于 form，因为 Switch 用 valuePropName）
  const [editStatus, setEditStatus] = useState<number>(1)

  // 重置表单 / 编辑模式填充
  useEffect(() => {
    if (open) {
      if (editMode && editSubscription) {
        form.setFieldsValue({
          target_price: editSubscription.target_price || undefined,
          remark: editSubscription.remark || '',
        })
        setEditStatus(editSubscription.status ?? 1)
      } else {
        form.resetFields()
        form.setFieldValue('email', user?.email || '')
        setSelectedCinema(null)
      }
    }
  }, [open, editMode, editSubscription, form, user])

  // 影院下拉选项
  const cinemaOptions = useMemo(() => {
    return cinemas.map(c => ({
      label: `${c.cinema_name}（均价 ¥${c.price.toFixed(1)}）`,
      value: c.cinema_name,
    }))
  }, [cinemas])

  // 选择影院时的处理
  const handleCinemaChange = (cinemaName: string) => {
    const cinema = cinemas.find(c => c.cinema_name === cinemaName) || null
    setSelectedCinema(cinema)
    form.setFieldValue('target_price', undefined)
  }

  // 目标票价上限 = 所选影院平均票价（创建模式用）
  const maxPrice = selectedCinema?.price || 0

  // 创建模式：校验目标票价
  const validateTargetPrice = (_rule: any, value: number) => {
    if (!selectedCinema) {
      return Promise.reject('请先选择影院')
    }
    if (value === undefined || value === null || value <= 0) {
      return Promise.reject('请输入目标票价')
    }
    if (maxPrice > 0 && value >= maxPrice) {
      return Promise.reject(`目标票价不能高于该影院平均票价 ¥${maxPrice.toFixed(1)}`)
    }
    return Promise.resolve()
  }

  // 编辑模式：校验目标票价（只能调低，允许等于初始值）
  const validateEditTargetPrice = (_rule: any, value: number) => {
    const initial = editSubscription?.initial_target_price || 0
    if (value === undefined || value === null || value <= 0) {
      return Promise.reject('请输入目标票价')
    }
    if (value > initial) {
      return Promise.reject(`目标票价不能高于初始值 ¥${initial.toFixed(1)}`)
    }
    return Promise.resolve()
  }

  // 提交
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()

      if (editMode && editSubscription) {
        // 编辑模式：更新目标票价、备注、状态
        const data: any = {}
        const oldPrice = Number(editSubscription.target_price) || 0
        const newPrice = Number(values.target_price) || 0
        if (newPrice > 0 && newPrice !== oldPrice) {
          data.target_price = newPrice
        }
        if (values.remark != null) data.remark = values.remark
        // 状态变化时才传
        if (editStatus !== editSubscription.status) {
          data.status = editStatus
        }

        const res = await updateSubscription(String(editSubscription.id), data)
        if (res.code === 0) {
          message.success('订阅更新成功')
        } else {
          message.error(res.msg || '更新失败')
          return
        }
      } else {
        // 创建模式
        const selectedCinemaName: string = values.cinema_name
        const cinemaInfo = cinemas.find(c => c.cinema_name === selectedCinemaName)
        if (!cinemaInfo || !cinemaInfo.cinema_id) {
          message.warning('未找到影院 ID')
          return
        }

        const email: string = (values.email || '').trim()
        if (!email) {
          message.warning('通知邮箱缺失，请重新登录')
          return
        }

        const res = await createSubscription({
          cinema_id: cinemaInfo.cinema_id,
          cinema_name: cinemaInfo.cinema_name,
          maoyan_city_id: cityId,
          movie_id: movieId ? String(movieId) : '',
          movie_name: movieName || '',
          email: email,
          target_price: values.target_price || 0,
          remark: values.remark || '',
        })
        if (res.code === 0) {
          message.success(res.data?.message || '订阅创建成功')
        } else {
          message.error(res.msg || '创建失败')
          return
        }
      }

      onSuccess?.()
    } catch (err: any) {
      if (err?.errorFields) return
      message.error(err?.message || '操作失败')
    }
  }

  // 删除订阅
  const handleDelete = async () => {
    if (!editSubscription) return
    try {
      const res = await deleteSubscription(String(editSubscription.id))
      if (res.code === 0) {
        message.success('订阅已删除')
        onSuccess?.()
      } else {
        message.error(res.msg || '删除失败')
      }
    } catch (err: any) {
      message.error(err?.message || '删除失败')
    }
  }

  return (
    <Drawer
      title={editMode ? '编辑订阅' : '创建订阅'}
      open={open}
      onClose={onClose}
      width={480}
      styles={{
        body: { background: '#1a1a2e', padding: 24 },
        header: { background: '#1a1a2e', borderBottom: '1px solid rgba(255,255,255,0.08)' },
      }}
      footer={
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          {/* 左侧：删除按钮（仅编辑模式） */}
          {editMode ? (
            <Popconfirm
              title="确定删除此订阅？"
              description="删除后不可恢复"
              onConfirm={handleDelete}
              okText="删除"
              cancelText="取消"
              okButtonProps={{ danger: true }}
            >
              <Button danger type="text" style={{ color: '#ff4d4f' }}>
                删除订阅
              </Button>
            </Popconfirm>
          ) : (
            <span />
          )}
          {/* 右侧：取消 + 保存 */}
          <Space>
            <Button onClick={onClose}>取消</Button>
            <Button type="primary" onClick={handleSubmit}>
              {editMode ? '保存修改' : '创建订阅'}
            </Button>
          </Space>
        </div>
      }
      footerStyle={{
        background: '#1a1a2e',
        borderTop: '1px solid rgba(255,255,255,0.08)',
      }}
    >
      <Form
        form={form}
        layout="vertical"
        style={{ color: '#fff' }}
      >
        {/* ========== 城市（只读，创建模式） ========== */}
        {!editMode && cityName && (
          <Form.Item label={<Text style={{ color: 'rgba(255,255,255,0.85)' }}>城市</Text>}>
            <Input
              value={cityName}
              readOnly
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid rgba(255,255,255,0.08)',
                color: 'rgba(255,255,255,0.65)',
              }}
            />
          </Form.Item>
        )}

        {/* ========== 影院选择 ========== */}
        {editMode ? (
          <Form.Item label={<Text style={{ color: 'rgba(255,255,255,0.85)' }}>影院</Text>}>
            <Input
              value={editSubscription?.cinema_name || editSubscription?.cinema?.name || '未知影院'}
              readOnly
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid rgba(255,255,255,0.08)',
                color: 'rgba(255,255,255,0.65)',
              }}
            />
          </Form.Item>
        ) : (
          <Form.Item
            name="cinema_name"
            label={<Text style={{ color: 'rgba(255,255,255,0.85)' }}>选择影院</Text>}
            rules={[{ required: true, message: '请选择影院' }]}
          >
            <Select
              placeholder="请选择要订阅的影院"
              options={cinemaOptions}
              onChange={handleCinemaChange}
              showSearch
              optionFilterProp="label"
              style={{ width: '100%' }}
              dropdownStyle={{ background: '#1a1a2e' }}
            />
          </Form.Item>
        )}

        {/* ========== 通知邮箱（仅创建模式，默认当前用户，只读） ========== */}
        {!editMode && (
          <>
            <Divider style={{ borderColor: 'rgba(255,255,255,0.08)', margin: '8px 0' }} />
            <Form.Item
              name="email"
              label={<Text style={{ color: 'rgba(255,255,255,0.85)' }}>通知邮箱</Text>}
              rules={[
                { required: true, message: '请输入通知邮箱' },
                { type: 'email', message: '请输入有效的邮箱地址' },
              ]}
            >
              <Input
                readOnly
                style={{
                  background: 'rgba(255,255,255,0.04)',
                  border: '1px solid rgba(255,255,255,0.08)',
                  color: 'rgba(255,255,255,0.65)',
                  cursor: 'default',
                }}
              />
            </Form.Item>
          </>
        )}

        {/* ========== 票价触发价格 ========== */}
        <Divider style={{ borderColor: 'rgba(255,255,255,0.08)', margin: '8px 0' }} />
        {editMode ? (
          <>
            <Form.Item
              name="target_price"
              label={<Text style={{ color: 'rgba(255,255,255,0.85)' }}>目标票价（¥）</Text>}
              rules={[{ required: true, validator: validateEditTargetPrice }]}
            >
              <InputNumber
                prefix="¥"
                min={0.1}
                max={editSubscription?.initial_target_price || undefined}
                step={0.1}
                style={{ width: '100%' }}
                size="large"
              />
            </Form.Item>
            <Text type="secondary" style={{ fontSize: 12, marginTop: -12, display: 'block', marginBottom: 16 }}>
              目标票价不能高于初始值 ¥{(editSubscription?.initial_target_price || 0).toFixed(1)}
            </Text>
          </>
        ) : (
          <>
            <Form.Item
              name="target_price"
              label={<Text style={{ color: 'rgba(255,255,255,0.85)' }}>目标票价（¥）</Text>}
              rules={[{ required: true, validator: validateTargetPrice }]}
            >
              <InputNumber
                prefix="¥"
                placeholder={selectedCinema ? `建议低于 ¥${maxPrice.toFixed(1)}` : '请先选择影院'}
                min={0.1}
                step={0.1}
                style={{ width: '100%' }}
                size="large"
                disabled={!selectedCinema}
              />
            </Form.Item>
            <Text type="secondary" style={{ fontSize: 12, marginTop: -12, display: 'block', marginBottom: 16 }}>
              {selectedCinema
                ? <>当{movieName ? `《${movieName}》` : '影院'}最低票价 ≤ 此价格时触发邮件通知（该影院均价 ¥{maxPrice.toFixed(1)}）</>
                : '请先选择影院后再填写目标票价'
              }
            </Text>
          </>
        )}

        {/* ========== 备注 ========== */}
        <Form.Item
          name="remark"
          label={<Text style={{ color: 'rgba(255,255,255,0.85)' }}>备注（可选）</Text>}
        >
          <Input.TextArea
            placeholder="添加备注信息"
            rows={2}
            style={{
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.08)',
              color: '#fff',
            }}
          />
        </Form.Item>

        {/* ========== 启用/禁用（仅编辑模式） ========== */}
        {editMode && (
          <>
            <Divider style={{ borderColor: 'rgba(255,255,255,0.08)', margin: '8px 0' }} />
            <Form.Item label={<Text style={{ color: 'rgba(255,255,255,0.85)' }}>订阅状态</Text>}>
              <Switch
                checked={editStatus === 1}
                checkedChildren="启用"
                unCheckedChildren="停用"
                onChange={(checked) => setEditStatus(checked ? 1 : 0)}
              />
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 12 }}>
                停用后将不再监控票价和发送通知
              </Text>
            </Form.Item>
          </>
        )}
      </Form>
    </Drawer>
  )
}
