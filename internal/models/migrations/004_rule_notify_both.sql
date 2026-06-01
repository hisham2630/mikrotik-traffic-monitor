ALTER TABLE alert_rules ADD COLUMN notify_whatsapp INTEGER NOT NULL DEFAULT 1;
ALTER TABLE alert_rules ADD COLUMN notify_telegram INTEGER NOT NULL DEFAULT 0;

UPDATE alert_rules SET notify_whatsapp = 0, notify_telegram = 1 WHERE notify_channel = 'telegram';
UPDATE alert_rules SET notify_whatsapp = 1, notify_telegram = 0 WHERE notify_channel = 'whatsapp' OR notify_channel IS NULL OR notify_channel = '';
