ALTER TABLE notification_config ADD COLUMN whatsapp_enabled INTEGER NOT NULL DEFAULT 0;
UPDATE notification_config SET whatsapp_enabled = enabled WHERE id = 1;

ALTER TABLE notification_config ADD COLUMN telegram_bot_token TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_config ADD COLUMN telegram_chat_ids TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_config ADD COLUMN telegram_enabled INTEGER NOT NULL DEFAULT 0;

ALTER TABLE alert_rules ADD COLUMN notify_channel TEXT NOT NULL DEFAULT 'whatsapp';

ALTER TABLE alert_history ADD COLUMN notify_channel TEXT;
