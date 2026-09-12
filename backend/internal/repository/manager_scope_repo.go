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
	sql sqlExecutor
}

// managerScopeUserWhere 是经理可见成员的统一过滤条件：
// 用户主部门命中该经理负责的部门，且用户至少有一条未删除的订阅记录。
const managerScopeUserWhere = `
	FROM users AS u
	JOIN user_manager_departments AS md ON md.dept_id = u.primary_dept_id
	WHERE md.manager_user_id = $1
	  AND u.deleted_at IS NULL
	  AND EXISTS (
		  SELECT 1 FROM user_subscriptions AS us
		  WHERE us.user_id = u.id AND us.deleted_at IS NULL
	  )`

func NewManagerScopeRepository(sqlDB *sql.DB) service.ManagerScopeRepository {
	return &managerScopeRepository{sql: sqlDB}
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

func (r *managerScopeRepository) ListManagerDepartments(ctx context.Context, managerUserID int64) ([]service.DingTalkDepartment, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT d.dept_id, d.parent_id, d.name, d.is_active, d.synced_at, d.created_at, d.updated_at
		FROM dingtalk_departments AS d
		JOIN user_manager_departments AS md ON md.dept_id = d.dept_id
		WHERE md.manager_user_id = $1
		ORDER BY d.name, d.dept_id`, managerUserID)
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

	if _, err := r.sql.ExecContext(ctx, `DELETE FROM user_manager_departments WHERE manager_user_id = $1`, managerUserID); err != nil {
		return err
	}
	if len(deptIDs) == 0 {
		return nil
	}
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO user_manager_departments (manager_user_id, dept_id)
		SELECT $1, unnest($2::bigint[])
		ON CONFLICT (manager_user_id, dept_id) DO NOTHING`, managerUserID, pq.Array(deptIDs))
	return err
}

func (r *managerScopeRepository) IsUserInManagerScope(ctx context.Context, managerUserID, userID int64) (bool, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users AS u
			JOIN user_manager_departments AS md ON md.dept_id = u.primary_dept_id
			WHERE md.manager_user_id = $1
			  AND u.id = $2
			  AND u.deleted_at IS NULL
			  AND EXISTS (
				  SELECT 1 FROM user_subscriptions AS us
				  WHERE us.user_id = u.id AND us.deleted_at IS NULL
			  )
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
	rows, err := r.sql.QueryContext(ctx, `SELECT COUNT(DISTINCT u.id) `+managerScopeUserWhere, managerUserID)
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

	rows, err := r.sql.QueryContext(ctx, `
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
