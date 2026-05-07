// Package cmddeps defines the shared dependency struct passed to command sub-packages.
// It exists as a separate package to avoid circular imports between pkg/cmd and its sub-packages.
package cmddeps

import (
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/notify"
)

// Deps holds pointers to the shared runtime dependencies injected into command sub-packages.
// Pointer fields allow commands to read the current value at execution time, since
// Store/Actor/AgentModel/OutputJSON are populated in PersistentPreRunE (after init).
type Deps struct {
	Store      *dolt.Store
	Actor      *string
	AgentModel *string
	OutputJSON *bool
	Notifier   *notify.Notifier
}
