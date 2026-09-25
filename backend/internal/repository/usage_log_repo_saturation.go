package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/lib/pq"
)

// saturationUserPredicate renders the optional user-scope filter used by the
// saturation report. A nil/empty scope means every quota holder (admin view);
// a non-empty scope narrows to those user IDs (manager view).
func saturationUserPredicate(alias string, scopeUserIDs []int64, args []any) (string, []any) {
	if len(scopeUserIDs) == 0 {
		return "", args
	}
	// users 表主键是 id，usage_logs 表通过 user_id 关联用户。
	column := "id"
	if alias == "ul" {
		column = "user_id"
	}
	query := fmt.Sprintf(" AND %s.%s = ANY($%d)", alias, column, len(args)+1)
	args = append(args, pq.Array(scopeUserIDs))
	return query, args
}

// GetSaturationUserAggregates returns one row per active quota holder: quota and
// current-window usage from user_subscriptions plus cumulative usage and
// activity from usage_logs. Cumulative totals are read from usage_logs instead
// of the window counters so a window reset (periodic or manual) cannot erase
// history.
func (r *usageLogRepository) GetSaturationUserAggregates(ctx context.Context, scopeUserIDs []int64) (results []usagestats.SaturationUserAggregate, err error) {
	query := `
		SELECT
			u.id,
			u.email,
			u.username,
			u.status,
			u.primary_dept_id,
			COALESCE(d.name, '') AS dept_name,
			COALESCE(STRING_AGG(DISTINCT g.name, ', '), '') AS group_names,
			COUNT(us.id) AS subscription_count,
			COALESCE(SUM(g.monthly_limit_usd), 0) AS quota_usd,
			COALESCE(SUM(us.monthly_usage_usd), 0) AS used_usd,
			COALESCE(SUM(us.daily_usage_usd), 0) AS daily_used_usd,
			COALESCE(SUM(us.weekly_usage_usd), 0) AS weekly_used_usd,
			MIN(us.monthly_window_start) AS window_started_at,
			MAX(us.expires_at) AS expires_at,
			COALESCE(lifetime.lifetime_used_usd, 0) AS lifetime_used_usd,
			COALESCE(lifetime.lifetime_requests, 0) AS lifetime_requests,
			lifetime.first_used_at,
			lifetime.last_used_at
		FROM users u
		JOIN user_subscriptions us
			ON us.user_id = u.id
			AND us.deleted_at IS NULL
			AND us.status = 'active'
		JOIN groups g
			ON g.id = us.group_id
			AND g.deleted_at IS NULL
			AND g.monthly_limit_usd > 0
		LEFT JOIN dingtalk_departments d ON d.dept_id = u.primary_dept_id
		LEFT JOIN LATERAL (
			SELECT
				COALESCE(SUM(ul.actual_cost), 0) AS lifetime_used_usd,
				COUNT(*) AS lifetime_requests,
				MIN(ul.created_at) AS first_used_at,
				MAX(ul.created_at) AS last_used_at
			FROM usage_logs ul
			WHERE ul.user_id = u.id
				AND ul.subscription_id IS NOT NULL
		) lifetime ON TRUE
		WHERE u.deleted_at IS NULL`

	args := []any{}
	var filter string
	filter, args = saturationUserPredicate("u", scopeUserIDs, args)
	query += filter

	query += `
		GROUP BY u.id, u.email, u.username, u.status, u.primary_dept_id, d.name,
			lifetime.lifetime_used_usd, lifetime.lifetime_requests, lifetime.first_used_at, lifetime.last_used_at
		ORDER BY u.id ASC`

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			results = nil
		}
	}()

	results = make([]usagestats.SaturationUserAggregate, 0)
	for rows.Next() {
		var row usagestats.SaturationUserAggregate
		var primaryDeptID sql.NullInt64
		var windowStartedAt sql.NullTime
		var expiresAt sql.NullTime
		var firstUsedAt sql.NullTime
		var lastUsedAt sql.NullTime
		if err := rows.Scan(
			&row.UserID,
			&row.Email,
			&row.Username,
			&row.Status,
			&primaryDeptID,
			&row.DeptName,
			&row.GroupNames,
			&row.SubscriptionCount,
			&row.QuotaUSD,
			&row.UsedUSD,
			&row.DailyUsedUSD,
			&row.WeeklyUsedUSD,
			&windowStartedAt,
			&expiresAt,
			&row.LifetimeUsedUSD,
			&row.LifetimeRequests,
			&firstUsedAt,
			&lastUsedAt,
		); err != nil {
			return nil, err
		}
		if primaryDeptID.Valid {
			deptID := primaryDeptID.Int64
			row.PrimaryDeptID = &deptID
		}
		if windowStartedAt.Valid {
			startedAt := windowStartedAt.Time
			row.WindowStartedAt = &startedAt
		}
		if expiresAt.Valid {
			expires := expiresAt.Time
			row.ExpiresAt = &expires
		}
		if firstUsedAt.Valid {
			firstUsed := firstUsedAt.Time
			row.FirstUsedAt = &firstUsed
		}
		if lastUsedAt.Valid {
			lastUsed := lastUsedAt.Time
			row.LastUsedAt = &lastUsed
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetSaturationUserCombos returns the current-window usage split by user, model
// and reasoning effort. Only subscription-billed rows that actually charged the
// quota (actual_cost > 0) are counted; the window boundary follows each
// subscription's monthly_window_start so resets take effect immediately.
func (r *usageLogRepository) GetSaturationUserCombos(ctx context.Context, scopeUserIDs []int64) (results []usagestats.SaturationUserCombo, err error) {
	modelExpr := resolveModelDimensionExpressionWithAlias(usagestats.ModelSourceRequested, "ul")
	effortExpr := `COALESCE(NULLIF(TRIM(ul.requested_reasoning_effort), ''), NULLIF(TRIM(ul.reasoning_effort), ''), 'unspecified')`

	query := fmt.Sprintf(`
		SELECT
			ul.user_id,
			%s AS model,
			%s AS effort,
			COUNT(*) AS requests,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS tokens,
			COALESCE(SUM(ul.actual_cost), 0) AS cost_usd
		FROM usage_logs ul
		JOIN user_subscriptions us
			ON us.id = ul.subscription_id
			AND us.deleted_at IS NULL
			AND us.status = 'active'
		JOIN groups g
			ON g.id = us.group_id
			AND g.deleted_at IS NULL
			AND g.monthly_limit_usd > 0
		WHERE ul.created_at >= us.monthly_window_start
			AND ul.actual_cost > 0`, modelExpr, effortExpr)

	args := []any{}
	filter, args := saturationUserPredicate("ul", scopeUserIDs, args)
	query += filter
	// NOTE: GROUP BY 必须重复完整的表达式文本，不能写别名 model。
	// PostgreSQL 在输入列与输出别名同名时优先解析为输入列，
	// GROUP BY model 会被解析成 ul.model，导致 SELECT 里的
	// ul.requested_model 报 "must appear in the GROUP BY clause" 500。
	query += fmt.Sprintf(" GROUP BY ul.user_id, %s, %s ORDER BY ul.user_id ASC, tokens DESC", modelExpr, effortExpr)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			results = nil
		}
	}()

	results = make([]usagestats.SaturationUserCombo, 0)
	for rows.Next() {
		var row usagestats.SaturationUserCombo
		if err := rows.Scan(
			&row.UserID,
			&row.Model,
			&row.Effort,
			&row.Requests,
			&row.Tokens,
			&row.CostUSD,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetSaturationDailyTrend returns the subscription-billed usage per day for
// the last days days, backfilled with zero rows so the chart has no gaps. It
// reads usage_logs on purpose: window resets only move counters, the history
// stays intact. Only logs billed to a quota-limited group count toward the
// daily totals.
func (r *usageLogRepository) GetSaturationDailyTrend(ctx context.Context, days int) (results []usagestats.SaturationTrendPoint, err error) {
	query := `
		WITH days AS (
			SELECT (CURRENT_DATE - (g - 1) * INTERVAL '1 day')::date AS day
			FROM generate_series(1, $1) AS g
		),
		usage AS (
			SELECT
				ul.created_at::date AS day,
				COUNT(*) AS requests,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS tokens,
				COALESCE(SUM(ul.actual_cost), 0) AS cost_usd,
				COUNT(DISTINCT ul.user_id) AS users
			FROM usage_logs ul
			JOIN user_subscriptions us
				ON us.id = ul.subscription_id
				AND us.deleted_at IS NULL
				AND us.status = 'active'
			JOIN groups g
				ON g.id = us.group_id
				AND g.deleted_at IS NULL
				AND g.monthly_limit_usd > 0
			WHERE ul.created_at::date > CURRENT_DATE - $1
				AND ul.actual_cost > 0
			GROUP BY ul.created_at::date
		)
		SELECT
			TO_CHAR(days.day, 'YYYY-MM-DD') AS day,
			COALESCE(u.requests, 0) AS requests,
			COALESCE(u.tokens, 0) AS tokens,
			COALESCE(u.cost_usd, 0) AS cost_usd,
			COALESCE(u.users, 0) AS users
		FROM days
		LEFT JOIN usage u ON u.day = days.day
		ORDER BY days.day ASC`

	rows, err := r.sql.QueryContext(ctx, query, days)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			results = nil
		}
	}()

	results = make([]usagestats.SaturationTrendPoint, 0, days)
	for rows.Next() {
		var row usagestats.SaturationTrendPoint
		if err := rows.Scan(
			&row.Date,
			&row.Requests,
			&row.Tokens,
			&row.CostUSD,
			&row.Users,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
