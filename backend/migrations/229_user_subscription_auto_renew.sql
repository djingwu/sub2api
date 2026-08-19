-- 用户订阅自动续期开关：钉钉用户默认开启，到期时免费自动续期。
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS auto_renew BOOLEAN NOT NULL DEFAULT TRUE;