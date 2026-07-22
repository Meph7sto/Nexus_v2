package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUsageInteractionRepositoryExistingUsageLogIDsReturnsOnlyRetainedRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := &usageInteractionRepository{db: db}
	mock.ExpectQuery(`SELECT usage_log_id FROM usage_interactions WHERE usage_log_id IN \(\$1, \$2\)`).
		WithArgs(int64(12), int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"usage_log_id"}).AddRow(int64(44)))
	mock.ExpectClose()

	available, err := repo.ExistingUsageLogIDs(context.Background(), []int64{12, 44, 12, 0, -1})

	require.NoError(t, err)
	require.Equal(t, map[int64]struct{}{44: {}}, available)
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}
