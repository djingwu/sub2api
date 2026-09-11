-- 修复订阅自动续期默认值：
-- 建表默认虽为 TRUE，但 createSubscription 曾以 Go 零值 false 显式写入，导致新建订阅默认关闭。
-- 应用层已改为默认 true，此处统一回填存量订阅，并确保列默认值为 TRUE。
ALTER TABLE user_subscriptions ALTER COLUMN auto_renew SET DEFAULT TRUE;
UPDATE user_subscriptions SET auto_renew = TRUE WHERE auto_renew = FALSE;
