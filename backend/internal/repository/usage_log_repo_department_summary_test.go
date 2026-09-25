package repository

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

// summaryFILTERMatcher fails the expectation when the issued SQL puts FILTER
// anywhere but directly after the aggregate. PostgreSQL rejects
// COALESCE(SUM(...), 0) FILTER (...) with "syntax error at or near FILTER",
// and sqlmock alone cannot catch that, so the shape is asserted here by
// collecting every FUNC(...) FILTER occurrence and requiring a SUM among
// them: in the broken variant the token aggregates match no FUNC(...) FILTER
// at all (the FILTER sits after COALESCE's closing paren).
type summaryFILTERMatcher struct{}

var aggregateFILTERRef = regexp.MustCompile(`(?i)\b([A-Z_]+)\([^()]*\)\s*FILTER\s*\(WHERE`)

func (summaryFILTERMatcher) Match(_, actualSQL string) error {
	matches := aggregateFILTERRef.FindAllStringSubmatch(actualSQL, -1)
	var funcs []string
	for _, match := range matches {
		funcs = append(funcs, strings.ToUpper(match[1]))
		if strings.EqualFold(match[1], "COALESCE") {
			return fmt.Errorf("FILTER must follow the aggregate, never a COALESCE wrapper: %s", actualSQL)
		}
	}
	for _, fn := range funcs {
		if fn == "SUM" {
			return nil
		}
	}
	return fmt.Errorf("expected SUM (...) FILTER (WHERE ...) in query: %s", actualSQL)
}

// TestUsageLogDepartmentSummarySQLShape verifies the summary query keeps its
// aggregate/FILTER shape and maps both scopes onto the reconciliation fields.
// This runs without a database so FILTER syntax regressions surface in unit
// tests (integration tests need Docker and stay compile-only locally).
func TestUsageLogDepartmentSummarySQLShape(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(summaryFILTERMatcher{}))
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()
	repo := newUsageLogRepositoryWithSQL(nil, sqlDB)

	rows := sqlmock.NewRows([]string{
		"department_requests",
		"department_tokens",
		"department_groups",
		"department_users",
		"other_requests",
		"other_tokens",
		"other_groups",
		"other_users",
	}).AddRow(int64(10), int64(100), int64(3), int64(5), int64(7), int64(70), int64(2), int64(4))
	mock.ExpectQuery(`.*`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	ctx := context.Background()
	summary, err := repo.GetDepartmentUsageSummaryWithFilters(
		ctx,
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
		usagestats.UsageLogFilters{},
	)
	require.NoError(t, err)
	require.Equal(t, int64(10), summary.TotalRequests)
	require.Equal(t, int64(100), summary.TotalTokens)
	require.Equal(t, int64(3), summary.ActiveDepartments)
	require.Equal(t, int64(5), summary.ActiveUsers)
	require.Equal(t, int64(7), summary.OtherGroupRequests)
	require.Equal(t, int64(70), summary.OtherGroupTokens)
	require.Equal(t, int64(2), summary.OtherGroupCount)
	require.Equal(t, int64(4), summary.OtherGroupUsers)
	require.NoError(t, mock.ExpectationsWereMet())
}
