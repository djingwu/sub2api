-- 成本豁免审计列：被成本豁免的用户在落库时 actual_cost / total_cost 被归零，
-- 但其真实费用需保留供运营直接查库审计。这两列在归零“之前”写入真实金额，
-- 所有从 usage_logs 聚合的报表只读取 actual_cost / total_cost，因此不会被污染。
-- 非豁免用户的这两列恒为 0（其真实费用本就体现在 actual_cost 中）。
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS raw_actual_cost numeric(20,10) NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS raw_total_cost numeric(20,10) NOT NULL DEFAULT 0;

-- 仅对成本豁免用户的审计查询建立索引（WHERE raw_actual_cost > 0 覆盖豁免且已落真实金额的行）。
CREATE INDEX IF NOT EXISTS idx_usage_logs_raw_actual_cost_audit
    ON usage_logs (user_id, created_at)
    WHERE raw_actual_cost > 0;
