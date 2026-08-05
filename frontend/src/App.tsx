// App.tsx — 路由根组件（暗色主题）
import { Routes, Route } from 'react-router-dom'
import { Layout } from 'antd'
import { AuthProvider } from './store/auth'
import Navbar from './components/Navbar'
import HomePage from './pages/HomePage'
import LoginPage from './pages/LoginPage'
import MoviePricePage from './pages/MoviePricePage'
import SubscriptionsPage from './pages/SubscriptionsPage'
import SubscriptionDetailPage from './pages/SubscriptionDetailPage'
import CinemaCrawlRecordsPage from './pages/CinemaCrawlRecordsPage'
import SubscriptionHistoryPage from './pages/SubscriptionHistoryPage'
import PriceChangesPage from './pages/PriceChangesPage'

const { Content } = Layout

export default function App() {
  return (
    <AuthProvider>
      <Layout style={{ minHeight: '100vh', background: '#0a0a0a' }}>
        <Navbar />
        <Content>
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/movie/:id" element={<MoviePricePage />} />
            <Route path="/subscriptions" element={<SubscriptionsPage />} />
            <Route path="/subscription/:id" element={<SubscriptionDetailPage />} />
<Route path="/subscription/:id/records" element={<CinemaCrawlRecordsPage />} />
            <Route path="/logs" element={<SubscriptionHistoryPage />} />
            <Route path="/price-changes" element={<PriceChangesPage />} />
          </Routes>
        </Content>
      </Layout>
    </AuthProvider>
  )
}
