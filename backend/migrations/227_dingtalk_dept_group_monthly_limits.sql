-- 钉钉部门专属订阅分组：每月用量限额 $400。
--
-- 订阅模式（subscription_type='subscription'）的分组按限额控制用量，
-- 用户在分组内的消费按月度限额（monthly_limit_usd）封顶。
-- 幂等：重复执行结果一致。
UPDATE groups
SET monthly_limit_usd = 400.00
WHERE name IN (
    '移动应用部', '大数据部', '算法部', '平台运营部',
    '后端开发组', '前端开发组', '公共技术组', '创意类开发组', '消费类开发组'
)
  AND deleted_at IS NULL;
