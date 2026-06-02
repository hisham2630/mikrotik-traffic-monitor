const API = '/api';

/** API list endpoints must be arrays; Go nil slices serialize as null. */
export function ensureArray(data) {
  return Array.isArray(data) ? data : [];
}

async function request(path, options = {}) {
  const res = await fetch(`${API}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

export const auth = {
  login: (username, password) =>
    request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  me: () => request('/auth/me'),
  changePassword: (password) =>
    request('/auth/change-password', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request('/auth/logout', { method: 'POST' }),
  sessions: () => request('/auth/sessions'),
};

export const dashboard = {
  overview: () => request('/dashboard'),
};

export const devices = {
  list: () => request('/devices'),
  get: (id) => request(`/devices/${id}`),
  create: (body) => request('/devices', { method: 'POST', body: JSON.stringify(body) }),
  update: (id, body) => request(`/devices/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  delete: (id) => request(`/devices/${id}`, { method: 'DELETE' }),
  copy: (id, body) => request(`/devices/${id}/copy`, { method: 'POST', body: JSON.stringify(body) }),
  test: (id, body) => request(`/devices/${id}/test`, { method: 'POST', body: JSON.stringify(body || {}) }),
  discover: (id) => request(`/devices/${id}/discover`),
  previewDiscover: (body) =>
    request('/devices/preview-discover', { method: 'POST', body: JSON.stringify(body) }),
  interfaces: (id) => request(`/devices/${id}/interfaces`),
  setInterfaces: (id, interfaces) =>
    request(`/devices/${id}/interfaces`, { method: 'PUT', body: JSON.stringify({ interfaces }) }),
  history: (id, iface, hours = 1) =>
    request(`/devices/${id}/history?interface=${encodeURIComponent(iface)}&hours=${hours}`),
  reboot: (id) => request(`/devices/${id}/reboot`, { method: 'POST' }),
};

export const rules = {
  list: () => request('/alert-rules'),
  create: (body) => request('/alert-rules', { method: 'POST', body: JSON.stringify(body) }),
  update: (id, body) => request(`/alert-rules/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  delete: (id) => request(`/alert-rules/${id}`, { method: 'DELETE' }),
};

export const settings = {
  notification: () => request('/settings/notification'),
  updateNotification: (body) =>
    request('/settings/notification', { method: 'PUT', body: JSON.stringify(body) }),
  testNotificationWhatsApp: () => request('/settings/notification/test/whatsapp', { method: 'POST' }),
  testNotificationTelegram: () => request('/settings/notification/test/telegram', { method: 'POST' }),
  app: () => request('/settings/app'),
  updateApp: (body) => request('/settings/app', { method: 'PUT', body: JSON.stringify(body) }),
};

export const users = {
  list: () => request('/users'),
  create: (body) => request('/users', { method: 'POST', body: JSON.stringify(body) }),
  delete: (id) => request(`/users/${id}`, { method: 'DELETE' }),
  updateRole: (id, role) =>
    request(`/users/${id}/role`, { method: 'PUT', body: JSON.stringify({ role }) }),
};

export const alerts = {
  history: () => request('/alert-history'),
};

export function formatBps(bps) {
  if (!bps) return '0';
  if (bps >= 1e9) return `${(bps / 1e9).toFixed(1)}G`;
  if (bps >= 1e6) return `${(bps / 1e6).toFixed(1)}M`;
  if (bps >= 1e3) return `${(bps / 1e3).toFixed(1)}K`;
  return `${bps}`;
}

/** Parse bandwidth strings like 10M, 1g, 23k, or plain bps numbers. Returns null if invalid. */
export function parseBps(input) {
  if (input == null || input === '') return null;
  const s = String(input).trim();
  if (!s) return null;
  const match = s.match(/^([\d.]+)\s*([kmg])?$/i);
  if (!match) return null;
  const num = parseFloat(match[1]);
  if (!Number.isFinite(num) || num <= 0) return null;
  const unit = (match[2] || '').toLowerCase();
  const mult = { k: 1e3, m: 1e6, g: 1e9 }[unit] ?? 1;
  return Math.round(num * mult);
}
