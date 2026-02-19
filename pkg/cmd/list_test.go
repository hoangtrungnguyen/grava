package cmd

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
)

func TestParseSortFlag(t *testing.T) {
	tests := []struct {
		name     string
		sortStr  string
		expected string
		wantErr  bool
	}{
		{
			name:     "Empty sort string (default)",
			sortStr:  "",
			expected: "priority ASC, created_at DESC, id ASC",
			wantErr:  false,
		},
		{
			name:     "Single field asc",
			sortStr:  "priority:asc",
			expected: "priority ASC, id ASC",
			wantErr:  false,
		},
		{
			name:     "Single field desc",
			sortStr:  "priority:desc",
			expected: "priority DESC, id ASC",
			wantErr:  false,
		},
		{
			name:     "Multiple fields",
			sortStr:  "priority:asc,created:desc",
			expected: "priority ASC, created_at DESC, id ASC",
			wantErr:  false,
		},
		{
			name:     "Format with spaces",
			sortStr:  " priority : asc , created : desc ",
			expected: "priority ASC, created_at DESC, id ASC",
			wantErr:  false,
		},
		{
			name:    "Invalid field",
			sortStr: "unknown:asc",
			wantErr: true,
		},
		{
			name:    "Invalid order",
			sortStr: "priority:up",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSortFlag(tt.sortStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSortFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parseSortFlag() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestListCmdSoftDelete(t *testing.T) {
	t.Run("excludes tombstones", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		// Reset flags
		listStatus = ""
		listType = ""
		listWisp = false
		listSort = ""

		expectedQuery := regexp.QuoteMeta(`SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 0 AND status != 'tombstone' ORDER BY priority ASC, created_at DESC, id ASC`)

		mock.ExpectQuery(expectedQuery).
			WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
				AddRow("grava-1", "Active Issue", "task", 1, "open", time.Now()))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "list")
		assert.NoError(t, err)
		assert.Contains(t, output, "Active Issue")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("list wisps excludes tombstones", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		listWisp = true
		defer func() { listWisp = false }()

		expectedQuery := regexp.QuoteMeta(`SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 1 AND status != 'tombstone' ORDER BY priority ASC, created_at DESC, id ASC`)

		mock.ExpectQuery(expectedQuery).
			WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}))

		mock.ExpectClose()

		_, err = executeCommand(rootCmd, "list", "--wisp")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
