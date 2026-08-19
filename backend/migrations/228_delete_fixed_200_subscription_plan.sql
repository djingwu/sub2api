-- 移除已废弃的固定 $200 套餐（FIXED_200_USD）。
--
-- 背景：FIXED_200_USD 套餐由迁移 225 播种，旧逻辑（注册自动绑定固定套餐）已移除，
-- 新的钉钉部门绑定逻辑（BindUserToDingTalkDeptGroup）不再依赖该套餐——
-- 订阅有效期改用分组的 default_validity_days，额度走分组 monthly_limit_usd。
-- 删除套餐行，保留其同名分组（'200 USD 固定订阅'，历史遗留，不影响功能）。
--
-- 幂等：重复执行不报错（IF EXISTS）。

DELETE FROM subscription_plans WHERE product_name = 'FIXED_200_USD';