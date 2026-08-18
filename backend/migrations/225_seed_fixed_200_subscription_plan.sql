-- 新增固定 $200 订阅套餐及其专用订阅分组（幂等种子数据）。
--
-- 站点固定订阅套餐：
--   - 分组 subscription_type='subscription'，是套餐绑定额度的载体
--   - 套餐 product_name='FIXED_200_USD' 是代码中的固定标识
--     （见 AuthService.bindFixedSubscriptionPlanToNewUser：每个新用户注册/首次登录自动绑定）
--   - 售价 200.00 USD，有效期 30 天；分组限额可在管理后台按需调整

-- 1. 确保固定套餐专用订阅分组存在。
-- 注意：检查包含已软删行（不限定 deleted_at IS NULL），
-- 保证管理员删除该组（软删）后迁移不会将其复活。
INSERT INTO groups (name, description, platform, subscription_type, rate_multiplier, is_exclusive, status, default_validity_days, sort_order, created_at, updated_at)
SELECT '200 USD 固定订阅', 'Fixed $200 subscription plan group (auto-bound to every new user)', 'anthropic', 'subscription', 1.0, FALSE, 'active', 30, 0, NOW(), NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM groups WHERE name = '200 USD 固定订阅'
);

-- 2. 播种固定 $200 套餐（绑定到上述分组）
INSERT INTO subscription_plans (group_id, name, description, price, original_price, currency, validity_days, validity_unit, features, product_name, for_sale, sort_order, created_at, updated_at)
SELECT g.id, '$200 固定套餐', 'Fixed $200 subscription plan: auto-bound to every new user on signup, 30-day validity', 200.00, NULL, 'USD', g.default_validity_days, 'day', '', 'FIXED_200_USD', TRUE, 0, NOW(), NOW()
FROM groups g
WHERE g.name = '200 USD 固定订阅'
  AND g.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM subscription_plans WHERE product_name = 'FIXED_200_USD'
  );