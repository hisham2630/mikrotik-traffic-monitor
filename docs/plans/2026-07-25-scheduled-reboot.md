# Scheduled Reboot

## Understanding

- App-side schedule (monitor process must be running); one schedule per device; admin-only configure/fire path matches manual reboot.
- Days-of-week bitmask + one `HH:MM`; evaluated in **server local** timezone.
- 15-minute catch-up window after the slot; if the app is down longer, that slot is skipped.
- Toggle off keeps days/time; schedules default **off** after migration.
- Notify WhatsApp and/or Telegram (whichever channels are enabled in `notification_config`) on success **or** failure.

## Assumptions

- Few–dozens of devices; a 1-minute ticker is enough (no cron library).
- No reboot history UI, no RouterOS-native scheduler, no per-device timezone.
- A failed attempt still sets `reboot_last_run_at` so the same slot does not retry-storm.
- Manual reboot does **not** update `reboot_last_run_at`.

## Decision Log

| Decision | Alternatives | Why |
|----------|--------------|-----|
| App-side minute ticker | RouterOS scheduler | Reuses `Client.Reboot()`; one place for notify + poller reload |
| Columns on `devices` | Separate table / cron lib | YAGNI; one schedule per device |
| Server local TZ | Settings TZ / per-device TZ | Simplest; explicit product choice |
| 15-min catch-up `[slot, slot+15m)` | Skip missed / unbounded catch-up | Brief downtime only |
| Admin-only | Viewers edit | Matches manual reboot |
| Notify both outcomes | Silent / fail-only | Ops visibility |
| Additive `ALTER TABLE` DEFAULT 0 | Rewrite init schema | Auto-migrate; prod unchanged until enabled |

## Spec

### Schema (`005_reboot_schedule.sql`)

```sql
ALTER TABLE devices ADD COLUMN reboot_schedule_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE devices ADD COLUMN reboot_days INTEGER NOT NULL DEFAULT 0; -- Sun=1 … Sat=64
ALTER TABLE devices ADD COLUMN reboot_time TEXT NOT NULL DEFAULT '03:00';
ALTER TABLE devices ADD COLUMN reboot_last_run_at TEXT;
```

Embedded under `internal/models/migrations/`; mirrored in `db/migrations/`.

### Due logic

Pure helper: `Due(now, days, timeHHMM, lastRun, catchup) (slot, ok)`.

Fire when:

1. `days != 0` and `time` parses as `HH:MM`
2. `now` is in `[slot, slot+catchup)` for today’s slot (weekday bit set) **or** yesterday’s slot (midnight wrap)
3. `lastRun` is nil or strictly before `slot`

Device `enabled` + `reboot_schedule_enabled` are checked by the scheduler before calling `Due`.

### API

Extend `Device` / `DeviceInput` (no new routes). JSON fields:

- `reboot_schedule_enabled` (bool)
- `reboot_days` (int bitmask Sun=1 … Sat=64)
- `reboot_time` (string `HH:MM`)
- `reboot_last_run_at` (nullable; response only / set by scheduler)

Validation: if schedule enabled → at least one day bit and valid `HH:MM`.

### Reboot helper + scheduler + notify

- Extract shared reboot (EOF/connection drop = success + `Poller.ReloadDevice`) from HTTP `rebootDevice`.
- Minute ticker from `cmd/server/main.go` via `internal/rebootsched`.
- On due: set `reboot_last_run_at` to attempt time, reboot, `Alerter.Notify(...)` success or failure.

## Edge-case check (pre-flight)

| Case | Handling |
|------|----------|
| App down &gt; 15m | Missed; no fire (by design) |
| Midnight wrap (e.g. 23:55 → 00:05) | Also evaluate yesterday’s slot |
| Reboot fails / connection drops | Connection/EOF treated success (same as manual); other errors notified as failure; `last_run_at` already set |
| Double tick same minute | `last_run_at >= slot` blocks re-fire |
| Schedule enabled, days=0 | Rejected on create/update; scheduler skips `days==0` |
| Device disabled | Scheduler skips |
| Manual reboot | Does not touch `reboot_last_run_at` |
| Notify channels both off | `Notify` no-ops quietly |
| Copy device | New device gets DB defaults (schedule off); not copied |
