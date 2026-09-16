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

// appendDepartmentUsageFilterConditions applies the shared usage filter shape to
// an analytics query. Callers that serve team-facing aggregates pass an empty
// business filter set on purpose: accepting user/api-key/group filters there
// would let a member narrow the result to another team's data.
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
	query, args = appendUsageLogModelQueryFilter(query, args, filters.Model, filters.ModelFilterSource)
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
		LEFT JOIN groups g ON g.id = ul.group_id
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
			LEFT JOIN groups g ON g.id = ul.group_id
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
