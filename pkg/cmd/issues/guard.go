package issues

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

// guardNotArchived checks that the issue is not in archived or tombstone status.
// Returns nil if the issue is writable, or a GravaError if it is read-only.
func guardNotArchived(store dolt.Store, issueID string) error {
	var status string
	err := store.QueryRow("SELECT status FROM issues WHERE id = ?", issueID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return gravaerrors.New("ISSUE_NOT_FOUND",
				fmt.Sprintf("Issue %s not found", issueID), nil)
		}
		return gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
	}
	if status == "archived" || status == "tombstone" {
		return gravaerrors.New("ISSUE_READ_ONLY",
			fmt.Sprintf("cannot modify issue %s: status is %s", issueID, status), nil)
	}
	return nil
}
