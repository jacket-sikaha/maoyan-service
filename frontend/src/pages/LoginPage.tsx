// pages/LoginPage.tsx — 登录/注册表单 Ant Design 版
import { useState } from 'react'
import { useAuth } from '../store/auth'
import { Card, Form, Input, Button, Typography, Alert, Space, Divider } from 'antd'
import { MailOutlined, LockOutlined, UserOutlined } from '@ant-design/icons'

const { Title, Text } = Typography

export default function LoginPage() {
  const { login, register } = useAuth()
  const [isLogin, setIsLogin] = useState(true)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (values: { email: string; password: string }) => {
    setError('')
    setLoading(true)
    try {
      if (isLogin) await login(values.email, values.password)
      else await register(values.email, values.password)
      window.location.href = '/'
    } catch (err: any) {
      setError(err.message || '操作失败')
    } finally { setLoading(false) }
  }

  return (
    <div style={{ maxWidth: 420, margin: '80px auto', padding: '0 16px' }}>
      <Card>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <UserOutlined style={{ fontSize: 40, color: '#1677ff' }} />
          <Title level={3} style={{ marginTop: 12 }}>
            {isLogin ? '登录猫眼票价监控' : '注册账号'}
          </Title>
        </div>

        {error && <Alert message={error} type="error" showIcon style={{ marginBottom: 16 }} />}

        <Form layout="vertical" onFinish={handleSubmit} size="large">
          <Form.Item
            name="email"
            rules={[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '邮箱格式不正确' }]}
          >
            <Input prefix={<MailOutlined />} placeholder="your@email.com" />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[
              { required: true, message: '请输入密码' },
              ...(isLogin ? [] : [{ min: 6, message: '密码至少6位' }])
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder={isLogin ? '输入密码' : '至少6位'} />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              {isLogin ? '登录' : '注册'}
            </Button>
          </Form.Item>
        </Form>

        <Divider />
        <div style={{ textAlign: 'center' }}>
          <Text type="secondary">
            {isLogin ? '没有账号？' : '已有账号？'}
          </Text>
          <Button
            type="link"
            onClick={() => { setIsLogin(!isLogin); setError('') }}
          >
            {isLogin ? '立即注册' : '去登录'}
          </Button>
        </div>
      </Card>
    </div>
  )
}
