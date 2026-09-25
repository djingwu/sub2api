package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSaturationUserPredicateColumns guards the scope filter used by the
// saturation report: the users table is keyed by id while usage_logs rows
// reference users through user_id (usage_logs.id is the log primary key).
func TestSaturationUserPredicateColumns(t *testing.T) {
	filter, args := saturationUserPredicate("u", []int64{7}, nil)
	require.Equal(t, " AND u.id = ANY($1)", filter)
	require.Len(t, args, 1)

	filter, args = saturationUserPredicate("ul", []int64{7}, nil)
	require.Equal(t, " AND ul.user_id = ANY($1)", filter)
	require.Len(t, args, 1)

	filter, args = saturationUserPredicate("u", nil, nil)
	require.Equal(t, "", filter)
	require.Empty(t, args)
}
