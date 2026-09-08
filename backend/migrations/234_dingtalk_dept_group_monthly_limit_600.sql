-- 钉钉部门专属订阅分组：限额调整 $400→$600（月），周限额统一为 $300。
--
-- 订阅模式（subscription_type='subscription'）的分组按限额控制用量，
-- 用户在分组内的消费按日/周/月度限额封顶。
-- 幂等：重复执行结果一致。

UPDATE groups
SET monthly_limit_usd = 600.00,
    weekly_limit_usd = 300.00
WHERE name IN (
    '移动应用部', '大数据部', '算法部', '平台运营部',
    '后端开发组', '前端开发组', '公共技术组', '创意类开发组', '消费类开发组'
)
  AND deleted_at IS NULL;
