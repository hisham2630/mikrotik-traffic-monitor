import React from 'react';
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  Outlet,
  Link,
  useLocation,
} from 'react-router-dom';
import { Layout, Menu, Spin } from 'antd';
import {
  DashboardOutlined,
  HddOutlined,
  BellOutlined,
  SettingOutlined,
  LogoutOutlined,
} from '@ant-design/icons';
import { AuthProvider, useAuth } from './context/AuthContext';
import AdminRoute from './components/AdminRoute';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import DeviceDetail from './pages/DeviceDetail';
import Devices from './pages/Devices';
import AlertRules from './pages/AlertRules';
import Settings from './pages/Settings';
import ChangePassword from './pages/ChangePassword';

const { Header, Sider, Content } = Layout;

/** Keeps layout + route outlet mounted so refresh stays on the same URL. */
function ProtectedLayout() {
  const { user, loading, logout, isAdmin } = useAuth();
  const loc = useLocation();

  if (!loading && !user) {
    return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />;
  }
  if (!loading && user?.must_change_password) {
    return <Navigate to="/change-password" replace />;
  }

  const selectedKey = loc.pathname.startsWith('/devices')
    ? '/devices'
    : loc.pathname.startsWith('/alerts')
      ? '/alerts'
      : loc.pathname.startsWith('/settings')
        ? '/settings'
        : '/';

  const items = [
    { key: '/', icon: <DashboardOutlined />, label: <Link to="/">Dashboard</Link> },
    { key: '/devices', icon: <HddOutlined />, label: <Link to="/devices">Devices</Link> },
    { key: '/alerts', icon: <BellOutlined />, label: <Link to="/alerts">Alerts</Link> },
  ];
  if (isAdmin) {
    items.push({
      key: '/settings',
      icon: <SettingOutlined />,
      label: <Link to="/settings">Settings</Link>,
    });
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider width={140} theme="dark" collapsedWidth={48}>
        <div style={{ padding: '8px 12px', color: '#fff', fontSize: 11, fontWeight: 600 }}>
          MT Monitor
        </div>
        <Menu theme="dark" mode="inline" selectedKeys={[selectedKey]} items={items} />
      </Sider>
      <Layout>
        <Header
          style={{
            padding: '0 12px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            height: 36,
            lineHeight: '36px',
          }}
        >
          {user && (
            <>
              <span style={{ marginRight: 12, fontSize: 12 }}>
                {user.username} ({user.role})
              </span>
              <a onClick={logout} style={{ fontSize: 12 }}>
                <LogoutOutlined /> Logout
              </a>
            </>
          )}
        </Header>
        <Content className="page-content">
          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: 48 }}>
              <Spin />
            </div>
          ) : (
            <Outlet />
          )}
        </Content>
      </Layout>
    </Layout>
  );
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/change-password" element={<ChangePassword />} />
      <Route element={<ProtectedLayout />}>
        <Route index element={<Dashboard />} />
        <Route path="devices" element={<Devices />} />
        <Route path="devices/:id" element={<DeviceDetail />} />
        <Route path="alerts" element={<AlertRules />} />
        <Route
          path="settings"
          element={
            <AdminRoute>
              <Settings />
            </AdminRoute>
          }
        />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  );
}
