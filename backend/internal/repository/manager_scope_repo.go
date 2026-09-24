package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type managerScopeRepository struct {
	db  *sql.DB
	sql sqlExecutor
}

// managerDeptTreeCTE 递归展开经理负责的部门及其所有子孙部门。
// 从 user_manager_departments 出发，沿 dingtalk_departments.parent_id 向下遍历。
// WHERE d.dept_id <> d.parent_id 防止自引用死循环，PostgreSQL 递归上限兜底。
//
// 同时展开 manager_dept_groups CTE：把经理管辖的 dept_id 经
// dingtalk_dept_group_map 映射成订阅分组名，供订阅分组口径复用。
const managerDeptTreeCTE = `
WITH RECURSIVE dept_tree AS (
	SELECT dept_id
	FROM user_manager_departments
	WHERE manager_user_id = $1
	UNION ALL
	SELECT d.dept_id
	FROM dingtalk_departments d
	JOIN dept_tree dt ON d.parent_id = dt.dept_id
	WHERE d.dept_id <> d.parent_id
),
manager_dept_groups AS (
	SELECT DISTINCT m.gname AS group_name
	FROM user_manager_departments umd
	JOIN (
		SELECT (kv).key::bigint AS dept_id, (kv).value AS gname
		FROM settings s, LATERAL jsonb_each_text(s.value::jsonb) AS kv
		WHERE s.key = 'dingtalk_dept_group_map'
		  AND s.value IS JSON
	) m ON m.dept_id = umd.dept_id
	WHERE umd.manager_user_id = $1
)`

// managerScopeUserWhere 是经理可见成员的统一过滤条件（两条口径取并集）：
//
//  1. 主部门口径：用户 primary_dept_id 命中该经理负责的部门（含子孙部门）；
//  2. 订阅分组口径：用户持有一条订阅，其分组名落在经理管辖部门映射出的
//     订阅分组集合里——primary_dept_id 缺失/越界的用户也能被看到。
//
// 两种口径都要求用户未删除且至少有一条未删除订阅。
// 注意：调用方需先拼接 managerDeptTreeCTE，本片段依赖 dept_tree/manager_dept_groups CTE。
const managerScopeUserWhere = `
	FROM users AS u
	WHERE u.deleted_at IS NULL
	  AND EXISTS (
		  SELECT 1 FROM user_subscriptions AS us
		  WHERE us.user_id = u.id AND us.deleted_at IS NULL
	  )
	  AND (
		u.primary_dept_id IN (SELECT dept_id FROM dept_tree)
		OR EXISTS (
			SELECT 1
			FROM user_subscriptions AS us2
			JOIN groups AS g ON g.id = us2.group_id AND g.deleted_at IS NULL
			WHERE us2.user_id = u.id
			  AND us2.deleted_at IS NULL
			  AND g.name IN (SELECT group_name FROM manager_dept_groups)
		)
	  )`

func NewManagerScopeRepository(sqlDB *sql.DB) service.ManagerScopeRepository {
	return &managerScopeRepository{db: sqlDB, sql: sqlDB}
}

func (r *managerScopeRepository) UpsertDepartment(ctx context.Context, department *service.DingTalkDepartment) error {
	if department == nil || department.DeptID <= 0 {
		return fmt.Errorf("department must have a positive dept_id")
	}
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO dingtalk_departments
			(dept_id, parent_id, name, is_active, synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (dept_id) DO UPDATE SET
			parent_id = EXCLUDED.parent_id,
			name = EXCLUDED.name,
			is_active = EXCLUDED.is_active,
			synced_at = EXCLUDED.synced_at,
			updated_at = NOW()`,
		department.DeptID,
		department.ParentID,
		department.Name,
		department.IsActive,
		department.SyncedAt,
	)
	return err
}

func (r *managerScopeRepository) ListDepartments(ctx context.Context) ([]service.DingTalkDepartment, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT dept_id, parent_id, name, is_active, synced_at, created_at, updated_at
		FROM dingtalk_departments
		ORDER BY name, dept_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	departments := make([]service.DingTalkDepartment, 0)
	for rows.Next() {
		var department service.DingTalkDepartment
		if err := rows.Scan(
			&department.DeptID,
			&department.ParentID,
			&department.Name,
			&department.IsActive,
			&department.SyncedAt,
			&department.CreatedAt,
			&department.UpdatedAt,
		); err != nil {
			return nil, err
		}
		departments = append(departments, department)
	}
	return departments, rows.Err()
}

func (r *managerScopeRepository) ListManagerDepartmentIDs(ctx context.Context, managerUserID int64) ([]int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT dept_id
		FROM user_manager_departments
		WHERE manager_user_id = $1
		ORDER BY dept_id`, managerUserID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	deptIDs := make([]int64, 0)
	for rows.Next() {
		var deptID int64
		if err := rows.Scan(&deptID); err != nil {
			return nil, err
		}
		deptIDs = append(deptIDs, deptID)
	}
	return deptIDs, rows.Err()
}

func (r *managerScopeRepository) ReplaceManagerDepartments(ctx context.Context, managerUserID int64, deptIDs []int64) error {
	if managerUserID <= 0 {
		return fmt.Errorf("manager_user_id must be positive")
	}
	unique := make(map[int64]struct{}, len(deptIDs))
	for _, deptID := range deptIDs {
		if deptID > 0 {
			unique[deptID] = struct{}{}
		}
	}
	deduped := make([]int64, 0, len(unique))
	for deptID := range unique {
		deduped = append(deduped, deptID)
	}
	sort.Slice(deduped, func(i, j int) bool { return deduped[i] < deduped[j] })
	deptIDs = deduped

	return r.runInTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_manager_departments WHERE manager_user_id = $1`, managerUserID); err != nil {
			return err
		}
		if len(deptIDs) == 0 {
			return nil
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO user_manager_departments (manager_user_id, dept_id)
			SELECT $1, unnest($2::bigint[])
			ON CONFLICT (manager_user_id, dept_id) DO NOTHING`, managerUserID, pq.Array(deptIDs))
		return err
	})
}

func (r *managerScopeRepository) runInTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin manager scope transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *managerScopeRepository) IsUserInManagerScope(ctx context.Context, managerUserID, userID int64) (bool, error) {
	rows, err := r.sql.QueryContext(ctx, managerDeptTreeCTE+`
		SELECT EXISTS (
			SELECT 1
			`+managerScopeUserWhere+`
			  AND u.id = $2
		)`, managerUserID, userID)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return false, rows.Err()
	}
	var exists bool
	if err := rows.Scan(&exists); err != nil {
		return false, err
	}
	return exists, rows.Err()
}

func (r *managerScopeRepository) countManagerUsers(ctx context.Context, managerUserID int64) (int64, error) {
	rows, err := r.sql.QueryContext(ctx, managerDeptTreeCTE+`
		SELECT COUNT(DISTINCT u.id) `+managerScopeUserWhere, managerUserID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var total int64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func (r *managerScopeRepository) ListManagerUserIDs(ctx context.Context, managerUserID int64, params pagination.PaginationParams) ([]int64, *pagination.PaginationResult, error) {
	total, err := r.countManagerUsers(ctx, managerUserID)
	if err != nil {
		return nil, nil, err
	}
	result := &pagination.PaginationResult{
		Total:    total,
		Page:     max(params.Page, 1),
		PageSize: params.Limit(),
	}
	if total > 0 {
		result.Pages = int((total + int64(result.PageSize) - 1) / int64(result.PageSize))
	}

	rows, err := r.sql.QueryContext(ctx, managerDeptTreeCTE+`
		SELECT DISTINCT u.id `+managerScopeUserWhere+`
		ORDER BY u.id DESC
		LIMIT $2 OFFSET $3`, managerUserID, result.PageSize, params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return ids, result, nil
}
