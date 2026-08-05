// components/Navbar.tsx — 深色主题导航栏
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../store/auth'
import { Layout, Space, Button, Typography } from 'antd'
import { HomeOutlined, BellOutlined, UserOutlined, LogoutOutlined, LoginOutlined } from '@ant-design/icons'

const { Header } = Layout
const { Text } = Typography

export default function Navbar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => { logout(); navigate('/') }

  return (
    <Header style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      background: 'rgba(10,10,10,0.95)', backdropFilter: 'blur(12px)',
      borderBottom: '1px solid rgba(255,255,255,0.06)',
      padding: '0 24px', position: 'sticky', top: 0, zIndex: 1000,
    }}>
      <Link to="/" style={{ textDecoration: 'none', display: 'flex', alignItems: 'center', gap: 8 }}>
        <Text style={{
          color: '#fff', fontSize: 20, fontWeight: 800,
          letterSpacing: -0.5,
        }}>🎬 猫眼票价监控</Text>
      </Link>
      <Space size="large">
        <Link to="/">
          <Button type="text" icon={<HomeOutlined />} style={{ color: 'rgba(255,255,255,0.65)' }}>首页</Button>
        </Link>
        <Link to="/subscriptions">
          <Button type="text" icon={<BellOutlined />} style={{ color: 'rgba(255,255,255,0.65)' }}>订阅管理</Button>
        </Link>
        <Link to="/price-changes">
          <Button type="text" icon={<BellOutlined />} style={{ color: 'rgba(255,255,255,0.65)' }}>票价变化</Button>
        </Link>
        <Link to="/logs">
          <Button type="text" icon={<BellOutlined />} style={{ color: 'rgba(255,255,255,0.65)' }}>执行日志</Button>
        </Link>
        {user ? (
          <>
            <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 13 }}>
              <UserOutlined style={{ marginRight: 4 }} />{user.email}
            </Text>
            <Button
              type="text" icon={<LogoutOutlined />}
              style={{ color: 'rgba(255,255,255,0.45)' }}
              onClick={handleLogout}
            >
              退出
            </Button>
          </>
        ) : (
          <Link to="/login">
            <Button type="text" icon={<LoginOutlined />} style={{ color: 'rgba(255,255,255,0.65)' }}>登录</Button>
          </Link>
        )}
      </Space>
    </Header>
  )
}
