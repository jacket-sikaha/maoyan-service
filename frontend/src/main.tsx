import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, HashRouter } from 'react-router-dom'
import { ConfigProvider, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import './index.css'

const MyRouter = import.meta.env.DEV ? BrowserRouter : HashRouter;




ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: '#1677ff',
          borderRadius: 8,
          colorBgContainer: '#1a1a2e',
          colorBgElevated: '#16213e',
        },
      }}
    >
      <MyRouter>
        <App />
      </MyRouter>
    </ConfigProvider>
  </React.StrictMode>,
)
