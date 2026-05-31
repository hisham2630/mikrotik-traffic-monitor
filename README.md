# MikroTik Traffic Monitor

Single-binary dashboard for monitoring MikroTik interface traffic with configurable alerts via HTTP notification API.

## Stack

- **Backend:** Go 1.22+, RouterOS API, SQLite, WebSocket
- **Frontend:** React 18, Vite, Ant Design (compact), uPlot (embedded in Go binary)

## Build

### Requirements

- Go 1.22+
- Node.js 16+ (for frontend build)

### Linux / Ubuntu 18 (deployment target)

```bash
make deps
make build
./mikrotik-monitor -listen :8081 -db /var/lib/mikrotik-monitor/data.db
```

### Windows (development)

```powershell
cd frontend
npm install
npm run build
cd ..
go mod tidy
go build -o mikrotik-monitor.exe ./cmd/server
.\mikrotik-monitor.exe -listen :8081
```

Copy `frontend/dist/*` to `internal/api/static/` before `go build` if not using Make on Windows.

## Sessions

- Sessions are stored in SQLite (`sessions` table), not only in cookies.
- Each login creates a **new session** (multi-session: phone + laptop + browser tabs stay independent until expiry or logout).
- Sessions survive server restarts (24h lifetime, pruned hourly).
- Logout revokes only the current session; other devices stay signed in.
- Password change revokes all other sessions but keeps the current browser logged in.

## First run

- Default login: `admin` / `admin` (password change required)
- Configure notification URL in Settings, e.g.:
  `http://62.72.20.125:3000/api/sendText?phone={phone}&text={message}&session=casher`
- Add MikroTik devices (API port 8728), fetch interfaces, select ports to monitor
- Create alert rules (threshold, duration, cooldown)

## Install (systemd)

On Linux, build and install the binary plus a systemd unit:

```bash
make deps
sudo make install
sudo nano /etc/mikrotik-monitor/env   # set MIKROTIK_MONITOR_SECRET
sudo systemctl enable --now mikrotik-monitor
```

Defaults:

| Path | Purpose |
|------|---------|
| `/opt/mikrotik-monitor/mikrotik-monitor` | Binary |
| `/var/lib/mikrotik-monitor/data.db` | SQLite database |
| `/etc/mikrotik-monitor/env` | Secret and optional overrides |
| `/etc/systemd/system/mikrotik-monitor.service` | systemd unit |

Override paths at install time, e.g. `sudo make install LISTEN=:9090`.

Remove the service (keeps data and config):

```bash
sudo make uninstall
```

## systemd unit (reference)

```ini
[Unit]
Description=MikroTik Traffic Monitor
After=network.target

[Service]
ExecStart=/opt/mikrotik-monitor/mikrotik-monitor -listen :8081 -db /var/lib/mikrotik-monitor/data.db
Restart=always
Environment=MIKROTIK_MONITOR_SECRET=your-long-random-secret

[Install]
WantedBy=multi-user.target
```

## Environment

| Flag / Env | Description |
|------------|-------------|
| `-listen` | HTTP listen address (default `:8081`) |
| `-db` | SQLite database path |
| `-secret` / `MIKROTIK_MONITOR_SECRET` | Encryption & JWT secret |
