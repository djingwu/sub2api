-- 钉钉部门专属订阅分组（幂等种子数据，可重复执行）。
--
-- 每个钉钉部门一个专属分组：
--   - subscription_type='subscription'：用户须持有该组有效订阅才能绑定 API Key
--     （见 APIKeyService.canUserBindGroup），订阅由登录同步逻辑按
--     settings.dingtalk_dept_group_map（dept_id → 部门名）自动绑定
--   - is_exclusive=TRUE：专属分组
--   - 分组账号池为空，需管理员把该部门使用的上游 gpt 账号挂到对应分组
--
-- 组名即部门名（与 dingtalk_dept_group_map 中的部门名一致）。

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '移动应用部', '钉钉移动应用部员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '移动应用部' AND deleted_at IS NULL);

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '大数据部', '钉钉大数据部员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '大数据部' AND deleted_at IS NULL);

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '算法部', '钉钉算法部员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '算法部' AND deleted_at IS NULL);

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '平台运营部', '钉钉平台运营部员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '平台运营部' AND deleted_at IS NULL);

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '后端开发组', '钉钉后端开发组员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '后端开发组' AND deleted_at IS NULL);

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '前端开发组', '钉钉前端开发组员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '前端开发组' AND deleted_at IS NULL);

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '公共技术组', '钉钉公共技术组员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '公共技术组' AND deleted_at IS NULL);

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '创意类开发组', '钉钉创意类开发组员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '创意类开发组' AND deleted_at IS NULL);

INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '消费类开发组', '钉钉消费类开发组员工专属订阅分组', 'openai', 'subscription', 1.0, TRUE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM groups WHERE name = '消费类开发组' AND deleted_at IS NULL);