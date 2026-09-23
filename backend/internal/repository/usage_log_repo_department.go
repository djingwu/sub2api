package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const departmentTopModelLimit = 5

// departmentTokenSum sums every token bucket for a usage_logs row. Keeping the
// expression in one place means the department table, the trend chart, and the
// heatmap all count "usage" the same way.
const departmentTokenSum = "COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)"

// departmentGroupScopeCondition keeps team-facing aggregates limited to real
// departments: active, non-deleted, exclusive DingTalk subscription groups.
// Standard groups such as 免费组 or cline and non-exclusive plan groups such as
// the fixed $200 subscription group are not departments, so their usage never
// shows up as a team row. The caller supplies the group_id column reference.
const departmentGroupScopeCondition = `%s IN (
			SELECT g.id
			FROM groups g
			WHERE g.deleted_at IS NULL
			  AND g.status = 'active'
			  AND g.subscription_type = 'subscription'
			  AND g.is_exclusive = TRUE
		)`

// appendDepartmentUsageFilterConditions applies the shared usage filter shape to
// an analytics query and restricts it to department groups (see
// departmentGroupScopeCondition). Callers that serve team-facing aggregates pass
// an empty business filter set on purpose: accepting user/api-key/group filters
// there would let a member narrow the result to another team's data.
func appendDepartmentUsageFilterConditions(query string, args []any, alias string, filters usagestats.UsageLogFilters) (string, []any) {
	column := func(name string) string {
		if alias == "" {
			return name
		}
		return alias + "." + name
	}
	if filters.UserID > 0 {
		query += fmt.Sprintf(" AND %s = $%d", column("user_id"), len(args)+1)
		args = append(args, filters.UserID)
	}
	if filters.APIKeyID > 0 {
		query += fmt.Sprintf(" AND %s = $%d", column("api_key_id"), len(args)+1)
		args = append(args, filters.APIKeyID)
	}
	if filters.AccountID > 0 {
		query += fmt.Sprintf(" AND %s = $%d", column("account_id"), len(args)+1)
		args = append(args, filters.AccountID)
	}
	if filters.GroupID > 0 {
		query += fmt.Sprintf(" AND %s = $%d", column("group_id"), len(args)+1)
		args = append(args, filters.GroupID)
	}
	query, args = appendUsageLogModelQueryFilterWithAlias(query, args, filters.Model, filters.ModelFilterSource, alias)
	query, args = appendRequestTypeOrStreamQueryFilter(query, args, filters.RequestType, filters.Stream)
	query, args = appendNativeCompactionV2QueryFilter(query, args, filters.NativeCompactionV2, alias)
	if filters.BillingType != nil {
		query += fmt.Sprintf(" AND %s = $%d", column("billing_type"), len(args)+1)
		args = append(args, int16(*filters.BillingType))
	}
	query, args = appendUsageLogBillingModeQueryFilter(query, args, filters.BillingMode, alias)
	if filters.UpstreamModelMismatch != nil {
		query += " AND " + upstreamModelMismatchCondition(column("upstream_model_mismatch"), *filters.UpstreamModelMismatch)
	}
	query += fmt.Sprintf(" AND "+departmentGroupScopeCondition, column("group_id"))
	return query, args
}

// GetGroupUsageBreakdownWithFilters returns the full token-only aggregate for
// every department, including request counts, the token mix, activity scale,
// multimodal output, and observability metrics. Cost columns are never selected.
func (r *usageLogRepository) GetGroupUsageBreakdownWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters) (results []usagestats.GroupUsageBreakdown, err error) {
	modelExpr := resolveModelDimensionExpressionWithAlias(usagestats.ModelSourceRequested, "ul")

	query := fmt.Sprintf(`
		SELECT
			COALESCE(ul.group_id, 0) AS group_id,
			COALESCE(g.name, '') AS group_name,
			COUNT(*) AS requests,
			%s AS total_tokens,
			COALESCE(SUM(ul.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(ul.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(ul.cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(ul.cache_read_tokens), 0) AS cache_read_tokens,
			COUNT(DISTINCT %s) AS model_count,
			COUNT(DISTINCT ul.user_id) AS active_user_count,
			COALESCE(SUM(ul.image_count), 0) AS image_count,
			COALESCE(SUM(ul.video_count), 0) AS video_count,
			COUNT(*) FILTER (WHERE ul.stream) AS stream_requests,
			COALESCE(AVG(ul.duration_ms) FILTER (WHERE ul.duration_ms IS NOT NULL AND %s), 0)::float8 AS avg_duration_ms,
			COALESCE(AVG(ul.first_token_ms) FILTER (WHERE ul.first_token_ms IS NOT NULL AND %s), 0)::float8 AS avg_first_token_ms
		FROM usage_logs ul
		LEFT JOIN groups g ON g.id = ul.group_id AND g.deleted_at IS NULL
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, departmentTokenSum, modelExpr, usageLogSuccessFilterUL, usageLogSuccessFilterUL)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	query += " GROUP BY ul.group_id, g.name ORDER BY group_id ASC"

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

	results = make([]usagestats.GroupUsageBreakdown, 0)
	for rows.Next() {
		var row usagestats.GroupUsageBreakdown
		if err := rows.Scan(
			&row.GroupID,
			&row.GroupName,
			&row.Requests,
			&row.TotalTokens,
			&row.InputTokens,
			&row.OutputTokens,
			&row.CacheCreationTokens,
			&row.CacheReadTokens,
			&row.ModelCount,
			&row.ActiveUserCount,
			&row.ImageCount,
			&row.VideoCount,
			&row.StreamRequests,
			&row.AvgDurationMs,
			&row.AvgFirstTokenMs,
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

// GetUsageTrendRawWithFilters aggregates token usage per time bucket directly
// from usage_logs, bypassing the pre-aggregated rollups. Department reports use
// the raw table so the trend chart and the department table always agree.
func (r *usageLogRepository) GetUsageTrendRawWithFilters(ctx context.Context, startTime, endTime time.Time, granularity string, filters usagestats.UsageLogFilters) (results []usagestats.DepartmentTrendPoint, err error) {
	dateFormat := safeDateFormat(granularity)

	query := fmt.Sprintf(`
		SELECT
			TO_CHAR(ul.created_at, '%s') AS bucket,
			COUNT(*) AS requests,
			%s AS total_tokens,
			COALESCE(SUM(ul.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(ul.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(ul.cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(ul.cache_read_tokens), 0) AS cache_read_tokens,
			COUNT(*) FILTER (WHERE ul.stream) AS stream_requests,
			COALESCE(SUM(ul.image_count), 0) AS image_count,
			COALESCE(SUM(ul.video_count), 0) AS video_count,
			COALESCE(AVG(ul.duration_ms) FILTER (WHERE ul.duration_ms IS NOT NULL AND %s), 0)::float8 AS avg_duration_ms,
			COALESCE(AVG(ul.first_token_ms) FILTER (WHERE ul.first_token_ms IS NOT NULL AND %s), 0)::float8 AS avg_first_token_ms
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, dateFormat, departmentTokenSum, usageLogSuccessFilterUL, usageLogSuccessFilterUL)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	query += " GROUP BY bucket ORDER BY bucket ASC"

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

	results = make([]usagestats.DepartmentTrendPoint, 0)
	for rows.Next() {
		var row usagestats.DepartmentTrendPoint
		if err := rows.Scan(
			&row.Bucket,
			&row.Requests,
			&row.TotalTokens,
			&row.InputTokens,
			&row.OutputTokens,
			&row.CacheCreationTokens,
			&row.CacheReadTokens,
			&row.StreamRequests,
			&row.ImageCount,
			&row.VideoCount,
			&row.AvgDurationMs,
			&row.AvgFirstTokenMs,
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

// GetUsageHeatmapWithFilters buckets token usage by weekday and hour in the
// requester's timezone. The timezone is a bound parameter, not interpolated, so
// it cannot be used for injection.
func (r *usageLogRepository) GetUsageHeatmapWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters, timezone string) (results []usagestats.UsageHeatmapPoint, err error) {
	where := " FROM usage_logs ul WHERE ul.created_at >= $1 AND ul.created_at < $2"
	args := []any{startTime, endTime}
	where, args = appendDepartmentUsageFilterConditions(where, args, "ul", filters)

	tzPlaceholder := fmt.Sprintf("$%d", len(args)+1)
	args = append(args, timezone)

	query := fmt.Sprintf(`
		SELECT
			EXTRACT(DOW FROM (ul.created_at AT TIME ZONE %s))::int AS weekday,
			EXTRACT(HOUR FROM (ul.created_at AT TIME ZONE %s))::int AS hour,
			COUNT(*) AS requests,
			%s AS total_tokens
	`, tzPlaceholder, tzPlaceholder, departmentTokenSum) + where + " GROUP BY weekday, hour ORDER BY weekday ASC, hour ASC"

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

	results = make([]usagestats.UsageHeatmapPoint, 0)
	for rows.Next() {
		var row usagestats.UsageHeatmapPoint
		if err := rows.Scan(&row.Weekday, &row.Hour, &row.Requests, &row.TotalTokens); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetModelUsageTrendWithFilters aggregates token usage per time bucket and
// model. The model dimension uses the requested-model source so the series
// matches what users actually asked for.
func (r *usageLogRepository) GetModelUsageTrendWithFilters(ctx context.Context, startTime, endTime time.Time, granularity string, filters usagestats.UsageLogFilters) (results []usagestats.ModelTrendPoint, err error) {
	dateFormat := safeDateFormat(granularity)
	modelExpr := resolveModelDimensionExpressionWithAlias(usagestats.ModelSourceRequested, "ul")

	query := fmt.Sprintf(`
		SELECT
			TO_CHAR(ul.created_at, '%s') AS bucket,
			%s AS model,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS total_tokens
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, dateFormat, modelExpr)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	query += fmt.Sprintf(" GROUP BY bucket, %s ORDER BY bucket ASC", modelExpr)

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

	results = make([]usagestats.ModelTrendPoint, 0)
	for rows.Next() {
		var row usagestats.ModelTrendPoint
		if err := rows.Scan(&row.Bucket, &row.Model, &row.TotalTokens); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetGroupModelStatsWithFilters returns the top N models by token usage inside
// each group, computed with a window function so the limit is per group rather
// than global. Cost columns are intentionally excluded.
func (r *usageLogRepository) GetGroupModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters, perGroupLimit int) (results []usagestats.GroupModelStat, err error) {
	if perGroupLimit <= 0 {
		perGroupLimit = departmentTopModelLimit
	}
	modelExpr := resolveModelDimensionExpressionWithAlias(usagestats.ModelSourceRequested, "ul")
	tokenSum := "COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)"

	query := fmt.Sprintf(`
		SELECT group_id, group_name, model, total_tokens
		FROM (
			SELECT
				COALESCE(ul.group_id, 0) AS group_id,
				COALESCE(g.name, '') AS group_name,
				%s AS model,
				%s AS total_tokens,
				ROW_NUMBER() OVER (
					PARTITION BY COALESCE(ul.group_id, 0)
					ORDER BY %s DESC, %s ASC
				) AS rn
			FROM usage_logs ul
			LEFT JOIN groups g ON g.id = ul.group_id AND g.deleted_at IS NULL
			WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, modelExpr, tokenSum, tokenSum, modelExpr)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	query += fmt.Sprintf(" GROUP BY ul.group_id, g.name, %s) ranked WHERE rn <= $%d ORDER BY group_id ASC, total_tokens DESC", modelExpr, len(args)+1)
	args = append(args, perGroupLimit)

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

	results = make([]usagestats.GroupModelStat, 0)
	for rows.Next() {
		var row usagestats.GroupModelStat
		if err := rows.Scan(&row.GroupID, &row.GroupName, &row.Model, &row.TotalTokens); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// departmentClientSoftwareExpr extracts a normalized client software name from
// the user-agent. The product is the part before the first "/" (or the whole
// string when no version is present), lowercased, stripped of anything outside
// [a-z0-9._-], and truncated so odd or oversized user-agents cannot leak host
// names or other fingerprints into the team report.
const departmentClientSoftwareExpr = `COALESCE(
	NULLIF(
		LEFT(
			REGEXP_REPLACE(
				CASE
					WHEN POSITION('/' IN ul.user_agent) > 0
					THEN LOWER(SUBSTRING(ul.user_agent FROM 1 FOR POSITION('/' IN ul.user_agent) - 1))
					ELSE LOWER(COALESCE(BTRIM(ul.user_agent), 'unknown'))
				END,
				'[^a-z0-9._-]', '', 'g'
			),
			32
		),
		''
	),
	'unknown'
)`

// GetClientSoftwareStatsWithFilters returns the top client software products
// (normalized user-agent product names) across the whole team, ordered by total
// tokens descending. Cost columns are never selected.
func (r *usageLogRepository) GetClientSoftwareStatsWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters, limit int) (results []usagestats.ClientSoftwareStat, err error) {
	if limit <= 0 {
		limit = 10
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS client_software,
			COUNT(*) AS requests,
			%s AS total_tokens,
			COUNT(DISTINCT ul.user_id) AS user_count,
			COUNT(DISTINCT COALESCE(ul.group_id, 0)) AS department_count
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, departmentClientSoftwareExpr, departmentTokenSum)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	query += fmt.Sprintf(`
		GROUP BY client_software
		ORDER BY total_tokens DESC, client_software ASC
		LIMIT $%d
	`, len(args)+1)
	args = append(args, limit)

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

	results = make([]usagestats.ClientSoftwareStat, 0)
	for rows.Next() {
		var row usagestats.ClientSoftwareStat
		if err := rows.Scan(&row.ClientSoftware, &row.Requests, &row.TotalTokens, &row.UserCount, &row.DepartmentCount); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetDepartmentModelStatsWithFilters returns the top models by token usage
// across every department. The model dimension uses the requested-model source
// so the report matches what users actually asked for, and cost columns are
// never selected.
func (r *usageLogRepository) GetDepartmentModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters, limit int) (results []usagestats.DepartmentModelStat, err error) {
	if limit <= 0 {
		limit = 10
	}
	modelExpr := resolveModelDimensionExpressionWithAlias(usagestats.ModelSourceRequested, "ul")

	query := fmt.Sprintf(`
		SELECT
			%s AS model,
			COUNT(*) AS requests,
			%s AS total_tokens,
			COUNT(DISTINCT ul.user_id) AS user_count
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, modelExpr, departmentTokenSum)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	// Positional GROUP BY/ORDER BY: "model" also exists as a usage_logs column,
	// so an unqualified reference would resolve to ul.model instead of the
	// requested-model alias above.
	query += fmt.Sprintf(`
		GROUP BY 1
		ORDER BY total_tokens DESC, 1 ASC
		LIMIT $%d
	`, len(args)+1)
	args = append(args, limit)

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

	results = make([]usagestats.DepartmentModelStat, 0)
	for rows.Next() {
		var row usagestats.DepartmentModelStat
		if err := rows.Scan(&row.Model, &row.Requests, &row.TotalTokens, &row.UserCount); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetDepartmentUsageSummaryWithFilters returns the cost-free headline totals for
// the selected range. ActiveUsers is a distinct-user count across all
// departments, so it never double counts someone active in two teams.
func (r *usageLogRepository) GetDepartmentUsageSummaryWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters) (*usagestats.DepartmentUsageSummary, error) {
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) AS total_requests,
			%s AS total_tokens,
			COUNT(DISTINCT COALESCE(ul.group_id, 0)) AS active_departments,
			COUNT(DISTINCT ul.user_id) AS active_users
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, departmentTokenSum)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	var summary usagestats.DepartmentUsageSummary
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return &summary, nil
	}
	if err := rows.Scan(
		&summary.TotalRequests,
		&summary.TotalTokens,
		&summary.ActiveDepartments,
		&summary.ActiveUsers,
	); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &summary, nil
}

// ListDepartmentGroups returns every real department (active, exclusive
// DingTalk subscription group) so the team usage report can show which
// departments had no usage in the selected range. Standard groups (免费组,
// cline, ...), non-exclusive plan groups, and soft-deleted groups are never
// reported.
func (r *usageLogRepository) ListDepartmentGroups(ctx context.Context) (results []usagestats.UnusedDepartment, err error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, COALESCE(name, '')
		FROM groups
		WHERE deleted_at IS NULL
		  AND status = 'active'
		  AND subscription_type = 'subscription'
		  AND is_exclusive = TRUE
		ORDER BY name ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			results = nil
		}
	}()

	results = make([]usagestats.UnusedDepartment, 0)
	for rows.Next() {
		var group usagestats.UnusedDepartment
		if err := rows.Scan(&group.GroupID, &group.GroupName); err != nil {
			return nil, err
		}
		results = append(results, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
