-- 用户成本豁免标记：被标记的用户在用量落库时成本字段归零，
-- 账面统计（用户消耗排行、分组日汇总、日报、利润预览等）不再体现其消耗，
-- 同时跳过余额/订阅/平台配额扣减（这些扣减均按 ActualCost > 0 守卫）。
ALTER TABLE users ADD COLUMN IF NOT EXISTS cost_exempt BOOLEAN NOT NULL DEFAULT false;
