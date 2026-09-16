package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// Reasoning effort reports are read-only aggregates over usage_logs. The two
// knobs below are allowlisted before they reach SQL so callers may pass user
// input safely: anything unknown falls back to the effective/reasoning default
// and the GPT-family model scope.
const (
	reasoningEffortSourceRequested = "requested"
	reasoningEffortModelScopeAll   = "all"
	reasoningEffortGPTPattern      = "gpt%"
)

// reasoningEffortColumn picks the column backing the effort dimension. The
// effective column is the value that actually ran (after mapping and the group
// ceiling); the requested column is what the client explicitly asked for and is
// empty whenever the client did not send an effort.
func reasoningEffortColumn(source string) string {
	if strings.EqualFold(strings.TrimSpace(source), reasoningEffortSourceRequested) {
		return "ul.requested_reasoning_effort"
	}
	return "ul.reasoning_effort"
}

// reasoningEffortBucketExpression normalizes the effort dimension into a stable
// label. Empty values become "unspecified" instead of dropping the rows, so the
// shares always add up to the full request count.
func reasoningEffortBucketExpression(source string) string {
	return fmt.Sprintf(
		"COALESCE(NULLIF(TRIM(%s), ''), '%s')",
		reasoningEffortColumn(source),
		usagestats.UnspecifiedReasoningEffort,
	)
}

// appendReasoningEffortModelScopeCondition restricts the report to GPT-family
// models unless the caller explicitly asked for every model. Other providers
// occasionally write a reasoning effort too, so this filter keeps the GPT view
// honest.
func appendReasoningEffortModelScopeCondition(query, modelScope string) string {
	if strings.EqualFold(strings.TrimSpace(modelScope), reasoningEffortModelScopeAll) {
		return query
	}
	return query + fmt.Sprintf(
		" AND (ul.model ILIKE '%s' OR ul.requested_model ILIKE '%s')",
		reasoningEffortGPTPattern,
		reasoningEffortGPTPattern,
	)
}

// queryReasoningEffortStats runs one effort aggregate and scans the shared
// column layout. Every query selects the same columns in the same order and
// fills unused dimensions with placeholders (0/”).
func (r *usageLogRepository) queryReasoningEffortStats(ctx context.Context, query string, args []any) (results []usagestats.ReasoningEffortStat, err error) {
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

	results = make([]usagestats.ReasoningEffortStat, 0)
	for rows.Next() {
		var row usagestats.ReasoningEffortStat
		if err := rows.Scan(
			&row.GroupID,
			&row.GroupName,
			&row.Model,
			&row.Bucket,
			&row.Effort,
			&row.Requests,
			&row.TotalTokens,
			&row.InputTokens,
			&row.OutputTokens,
			&row.CacheCreationTokens,
			&row.CacheReadTokens,
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

// reasoningEffortSelectColumns is the shared projection for every effort
// aggregate. Callers splice in the dimension expressions for group, model, and
// bucket before the effort expression.
const reasoningEffortSelectColumns = `
			%s AS group_id,
			%s AS group_name,
			%s AS model,
			%s AS bucket,
			%s AS effort,
			COUNT(*) AS requests,
			%s AS total_tokens,
			COALESCE(SUM(ul.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(ul.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(ul.cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(ul.cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(AVG(ul.duration_ms) FILTER (WHERE ul.duration_ms IS NOT NULL AND %s), 0)::float8 AS avg_duration_ms,
			COALESCE(AVG(ul.first_token_ms) FILTER (WHERE ul.first_token_ms IS NOT NULL AND %s), 0)::float8 AS avg_first_token_ms`

// GetReasoningEffortGroupStatsWithFilters aggregates reasoning-effort tiers per
// department, so the report can show one department in isolation or every
// department at once.
func (r *usageLogRepository) GetReasoningEffortGroupStatsWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters, source, modelScope string) ([]usagestats.ReasoningEffortStat, error) {
	effortExpr := reasoningEffortBucketExpression(source)

	columns := fmt.Sprintf(
		reasoningEffortSelectColumns,
		"COALESCE(ul.group_id, 0)",
		"COALESCE(g.name, '')",
		"''::text",
		"''::text",
		effortExpr,
		departmentTokenSum,
		usageLogSuccessFilterUL,
		usageLogSuccessFilterUL,
	)

	query := fmt.Sprintf(`
		SELECT %s
		FROM usage_logs ul
		LEFT JOIN groups g ON g.id = ul.group_id AND g.deleted_at IS NULL
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, columns)
	query = appendReasoningEffortModelScopeCondition(query, modelScope)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	query += fmt.Sprintf(" GROUP BY ul.group_id, g.name, %s ORDER BY group_id ASC, COUNT(*) DESC", effortExpr)

	return r.queryReasoningEffortStats(ctx, query, args)
}

// GetReasoningEffortModelStatsWithFilters aggregates reasoning-effort tiers per
// model. Combined with a group filter it answers "how does one department spend
// its effort per model".
func (r *usageLogRepository) GetReasoningEffortModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, filters usagestats.UsageLogFilters, source, modelScope string) ([]usagestats.ReasoningEffortStat, error) {
	effortExpr := reasoningEffortBucketExpression(source)
	modelExpr := resolveModelDimensionExpressionWithAlias(usagestats.ModelSourceRequested, "ul")

	columns := fmt.Sprintf(
		reasoningEffortSelectColumns,
		"0",
		"''::text",
		modelExpr,
		"''::text",
		effortExpr,
		departmentTokenSum,
		usageLogSuccessFilterUL,
		usageLogSuccessFilterUL,
	)

	query := fmt.Sprintf(`
		SELECT %s
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, columns)
	query = appendReasoningEffortModelScopeCondition(query, modelScope)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	query += fmt.Sprintf(" GROUP BY %s, %s ORDER BY COUNT(*) DESC", modelExpr, effortExpr)

	return r.queryReasoningEffortStats(ctx, query, args)
}

// GetReasoningEffortTrendWithFilters aggregates reasoning-effort tiers per time
// bucket, which shows how the effort mix shifts over the selected range.
func (r *usageLogRepository) GetReasoningEffortTrendWithFilters(ctx context.Context, startTime, endTime time.Time, granularity string, filters usagestats.UsageLogFilters, source, modelScope string) ([]usagestats.ReasoningEffortStat, error) {
	dateFormat := safeDateFormat(granularity)
	effortExpr := reasoningEffortBucketExpression(source)
	bucketExpr := fmt.Sprintf("TO_CHAR(ul.created_at, '%s')", dateFormat)

	columns := fmt.Sprintf(
		reasoningEffortSelectColumns,
		"0",
		"''::text",
		"''::text",
		bucketExpr,
		effortExpr,
		departmentTokenSum,
		usageLogSuccessFilterUL,
		usageLogSuccessFilterUL,
	)

	query := fmt.Sprintf(`
		SELECT %s
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
	`, columns)
	query = appendReasoningEffortModelScopeCondition(query, modelScope)

	args := []any{startTime, endTime}
	query, args = appendDepartmentUsageFilterConditions(query, args, "ul", filters)
	query += fmt.Sprintf(" GROUP BY %s, %s ORDER BY bucket ASC, COUNT(*) DESC", bucketExpr, effortExpr)

	return r.queryReasoningEffortStats(ctx, query, args)
}
