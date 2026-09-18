-- 经理负责部门不再局限于本地部门目录缓存。
--
-- 可选部门来自设置项 dingtalk_dept_group_map（dept_id → 订阅分组名），而
-- dingtalk_departments 只缓存钉钉登录时遇到的部门及其父级链，未必包含映射里的
-- 全部部门。原外键 (dept_id → dingtalk_departments.dept_id) 会让这类部门在保存
-- 经理范围时报 internal error，因此移除该外键，仅保留 dept_id 索引。
--
-- 授权判定只比对 users.primary_dept_id 与 user_manager_departments.dept_id，
-- 不依赖部门名称或目录缓存，去掉外键不影响权限正确性。
ALTER TABLE user_manager_departments
    DROP CONSTRAINT IF EXISTS user_manager_departments_dept_id_fkey;
