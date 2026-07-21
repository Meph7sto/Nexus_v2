package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryGetCurrentDatabaseSizeBytes(t *testing.T) {
	const query = "SELECT pg_database_size(current_database())"

	t.Run("returns reported size", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectQuery(regexp.QuoteMeta(query)).
			WillReturnRows(sqlmock.NewRows([]string{"pg_database_size"}).AddRow(int64(123456)))

		size, err := (&opsRepository{db: db}).GetCurrentDatabaseSizeBytes(context.Background())
		require.NoError(t, err)
		require.EqualValues(t, 123456, size)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	for _, value := range []any{nil, int64(-1)} {
		name := "null"
		if value != nil {
			name = "negative"
		}
		t.Run(name+" becomes zero", func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WillReturnRows(sqlmock.NewRows([]string{"pg_database_size"}).AddRow(value))

			size, err := (&opsRepository{db: db}).GetCurrentDatabaseSizeBytes(context.Background())
			require.NoError(t, err)
			require.Zero(t, size)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	t.Run("propagates query failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnError(errors.New("database unavailable"))

		_, err = (&opsRepository{db: db}).GetCurrentDatabaseSizeBytes(context.Background())
		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rejects nil repository", func(t *testing.T) {
		var repo *opsRepository
		_, err := repo.GetCurrentDatabaseSizeBytes(context.Background())
		require.Error(t, err)
	})
}
