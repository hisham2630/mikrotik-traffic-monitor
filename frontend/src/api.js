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

/** Reboot day bitmask: Sun=1, Mon=2, Tue=4, Wed=8, Thu=16, Fri=32, Sat=64 */
export const REBOOT_DAY_OPTIONS = [
  { label: 'Sun', value: 1 },
  { label: 'Mon', value: 2 },
  { label: 'Tue', value: 4 },
  { label: 'Wed', value: 8 },
  { label: 'Thu', value: 16 },
  { label: 'Fri', value: 32 },
  { label: 'Sat', value: 64 },
];

const REBOOT_DAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

export function encodeRebootDays(days) {
  if (typeof days === 'number') return days;
  return (Array.isArray(days) ? days : []).reduce((acc, bit) => acc | Number(bit), 0);
}

export function decodeRebootDays(bits) {
  const n = Number(bits) || 0;
  return REBOOT_DAY_OPTIONS.map((o) => o.value).filter((bit) => n & bit);
}

function normalizeRebootTime(t) {
  const m = String(t || '').match(/^(\d{1,2}):(\d{2})/);
  if (!m) return '03:00';
  return `${String(Number(m[1])).padStart(2, '0')}:${m[2]}`;
}

/** Build create/update body; maps form day checkboxes → reboot_days bitmask. */
export function deviceWriteBody(values) {
  const body = {
    ...values,
    reboot_schedule_enabled: !!values.reboot_schedule_enabled,
    reboot_days: encodeRebootDays(values.reboot_days),
    reboot_time: normalizeRebootTime(values.reboot_time),
  };
  delete body.reboot_last_run_at;
  return body;
}

/**
 * Client-side next slot label from days bitmask + HH:MM (local clock).
 * Returns e.g. "Next reboot: Sun 03:00", or null if no slot.
 */
export function formatNextReboot(daysBitmask, timeHHMM, now = new Date()) {
  const bits = Number(daysBitmask) || 0;
  if (!bits || !timeHHMM) return null;
  const m = String(timeHHMM).match(/^(\d{1,2}):(\d{2})/);
  if (!m) return null;
  const hh = Number(m[1]);
  const mm = Number(m[2]);
  if (hh > 23 || mm > 59) return null;
  for (let i = 0; i < 8; i++) {
    const d = new Date(now.getTime());
    d.setSeconds(0, 0);
    d.setMilliseconds(0);
    d.setDate(d.getDate() + i);
    d.setHours(hh, mm, 0, 0);
    const bit = 1 << d.getDay();
    if ((bits & bit) && d.getTime() > now.getTime()) {
      const t = `${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}`;
      return `Next reboot: ${REBOOT_DAY_NAMES[d.getDay()]} ${t}`;
    }
  }
  return null;
}

export const devices = {
  list: () => request('/devices'),
  get: (id) => request(`/devices/${id}`),
  create: (body) => request('/devices', { method: 'POST', body: JSON.stringify(deviceWriteBody(body)) }),
  update: (id, body) => request(`/devices/${id}`, { method: 'PUT', body: JSON.stringify(deviceWriteBody(body)) }),
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
