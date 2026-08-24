-- 233: 一次性历史数据修正（djw@longse.net）
--
-- 目标：把该用户历史用量按 /100 缩小、金额归零，与成本豁免开关（231）保持一致，
-- 使其历史消耗在报表中"看不到了/小到 1%"。
--
-- 影响范围：
--   1) usage_logs：该用户全部历史行的 token / 缓存 token / 图片·视频张数 ÷100，
--      所有金额列（含 total_cost / actual_cost / account_stats_cost）置 0。
--   2) 全局日/小时聚合表（usage_dashboard_daily / usage_dashboard_hourly）：
--      先按"原始 usage_logs"扣减该用户历史贡献的 99%（保留 1%），使历史总量也反映缩小后的用量。
--      注意：必须在改写 usage_logs 之前执行（本文件内顺序已保证）。
--
-- 由 migrations_runner 按文件名只执行一次；若目标用户不存在，各语句为 no-op，安全可重复部署。
-- 该文件仅做数据修正，不修改表结构；如需调整比例，请新建迁移文件，勿修改本文件。

-- 1) 全局日聚合：扣减该用户历史贡献的 99%
WITH t AS (
    SELECT
        ul.created_at::date AS bucket_date,
        sum(ul.input_tokens)::bigint AS in_t,
        sum(ul.output_tokens)::bigint AS out_t,
        sum(ul.cache_creation_tokens)::bigint AS cc_t,
        sum(ul.cache_read_tokens)::bigint AS cr_t,
        count(*)::bigint AS req,
        sum(ul.total_cost) AS tc,
        sum(ul.actual_cost) AS ac,
        sum(ul.duration_ms)::bigint AS dur
    FROM usage_logs ul
    WHERE ul.user_id = (SELECT id FROM users WHERE email = 'djw@longse.net')
    GROUP BY ul.created_at::date
)
UPDATE usage_dashboard_daily d
SET
    input_tokens = d.input_tokens - COALESCE((t.in_t * 99 / 100), 0),
    output_tokens = d.output_tokens - COALESCE((t.out_t * 99 / 100), 0),
    cache_creation_tokens = d.cache_creation_tokens - COALESCE((t.cc_t * 99 / 100), 0),
    cache_read_tokens = d.cache_read_tokens - COALESCE((t.cr_t * 99 / 100), 0),
    total_requests = d.total_requests - COALESCE((t.req * 99 / 100), 0),
    total_cost = d.total_cost - COALESCE(t.tc * 0.99, 0),
    actual_cost = d.actual_cost - COALESCE(t.ac * 0.99, 0),
    total_duration_ms = d.total_duration_ms - COALESCE((t.dur * 99 / 100), 0)
FROM t
WHERE d.bucket_date = t.bucket_date;

-- 2) 全局小时聚合：同上
WITH t AS (
    SELECT
        date_trunc('hour', ul.created_at) AS bucket_start,
        sum(ul.input_tokens)::bigint AS in_t,
        sum(ul.output_tokens)::bigint AS out_t,
        sum(ul.cache_creation_tokens)::bigint AS cc_t,
        sum(ul.cache_read_tokens)::bigint AS cr_t,
        count(*)::bigint AS req,
        sum(ul.total_cost) AS tc,
        sum(ul.actual_cost) AS ac,
        sum(ul.duration_ms)::bigint AS dur
    FROM usage_logs ul
    WHERE ul.user_id = (SELECT id FROM users WHERE email = 'djw@longse.net')
    GROUP BY date_trunc('hour', ul.created_at)
)
UPDATE usage_dashboard_hourly h
SET
    input_tokens = h.input_tokens - COALESCE((t.in_t * 99 / 100), 0),
    output_tokens = h.output_tokens - COALESCE((t.out_t * 99 / 100), 0),
    cache_creation_tokens = h.cache_creation_tokens - COALESCE((t.cc_t * 99 / 100), 0),
    cache_read_tokens = h.cache_read_tokens - COALESCE((t.cr_t * 99 / 100), 0),
    total_requests = h.total_requests - COALESCE((t.req * 99 / 100), 0),
    total_cost = h.total_cost - COALESCE(t.tc * 0.99, 0),
    actual_cost = h.actual_cost - COALESCE(t.ac * 0.99, 0),
    total_duration_ms = h.total_duration_ms - COALESCE((t.dur * 99 / 100), 0)
FROM t
WHERE h.bucket_start = t.bucket_start;

-- 3) 改写 usage_logs 本身：用量 ÷100，金额置 0
UPDATE usage_logs
SET
    input_tokens = input_tokens / 100,
    output_tokens = output_tokens / 100,
    cache_creation_tokens = cache_creation_tokens / 100,
    cache_read_tokens = cache_read_tokens / 100,
    cache_creation_5m_tokens = cache_creation_5m_tokens / 100,
    cache_creation_1h_tokens = cache_creation_1h_tokens / 100,
    image_output_tokens = image_output_tokens / 100,
    image_input_tokens = image_input_tokens / 100,
    image_count = image_count / 100,
    video_count = video_count / 100,
    input_cost = 0,
    output_cost = 0,
    cache_creation_cost = 0,
    cache_read_cost = 0,
    image_output_cost = 0,
    image_input_cost = 0,
    total_cost = 0,
    actual_cost = 0,
    account_stats_cost = 0
WHERE user_id = (SELECT id FROM users WHERE email = 'djw@longse.net');
