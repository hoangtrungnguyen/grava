package sandbox

import (
	"context"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmd/reserve"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

const ts08ID = "TS-08"

func init() {
	Register(Scenario{
		ID:       ts08ID,
		Name:     "File Reservation Enforcement",
		EpicGate: 8,
		Run:      runTS08,
	})
}

// runTS08 validates file reservation enforcement:
//  1. Agent-A declares exclusive lease on a path pattern
//  2. Agent-B's staged files are checked → conflict detected
//  3. Agent-A's own staged files pass → no conflict
//  4. Non-exclusive lease does not block
//  5. Release clears the block
func runTS08(ctx context.Context, store dolt.Store) Result {
	details := []string{}

	// Cleanup all reservations created by this test
	var reservationIDs []string
	defer func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, id := range reservationIDs {
			_ = reserve.ReleaseReservation(ctx2, store, id)
			_, _ = store.ExecContext(ctx2, "DELETE FROM file_reservations WHERE id = ?", id)
		}
	}()

	// 1. Agent-A declares exclusive lease
	result, err := reserve.DeclareReservation(ctx, store, reserve.DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "ts08-agent-A",
		Exclusive:   true,
		TTLMinutes:  5,
	})
	if err != nil {
		return fail(ts08ID, fmt.Sprintf("declare: %v", err))
	}
	reservationIDs = append(reservationIDs, result.Reservation.ID)
	details = append(details, fmt.Sprintf("agent-A declared exclusive lease: %s", result.Reservation.ID))

	// 2. Agent-B checks staged files → should detect conflict
	conflicts, err := reserve.CheckStagedConflicts(ctx, store,
		[]string{"src/cmd/issues/create.go"}, "ts08-agent-B")
	if err != nil {
		return fail(ts08ID, fmt.Sprintf("check conflicts (agent-B): %v", err), details...)
	}
	if len(conflicts) == 0 {
		return fail(ts08ID, "expected conflict for agent-B but got none", details...)
	}
	details = append(details, "agent-B correctly blocked by exclusive lease")

	// 3. Agent-A checks own staged files → should NOT conflict
	ownConflicts, err := reserve.CheckStagedConflicts(ctx, store,
		[]string{"src/cmd/issues/create.go"}, "ts08-agent-A")
	if err != nil {
		return fail(ts08ID, fmt.Sprintf("check conflicts (agent-A): %v", err), details...)
	}
	if len(ownConflicts) > 0 {
		return fail(ts08ID, "agent-A should not be blocked by own lease", details...)
	}
	details = append(details, "agent-A passes through own lease (AC#3)")

	// 4. Non-exclusive lease does not block
	sharedResult, err := reserve.DeclareReservation(ctx, store, reserve.DeclareParams{
		PathPattern: "src/shared/*.go",
		AgentID:     "ts08-agent-C",
		Exclusive:   false,
		TTLMinutes:  5,
	})
	if err != nil {
		return fail(ts08ID, fmt.Sprintf("declare shared: %v", err), details...)
	}
	reservationIDs = append(reservationIDs, sharedResult.Reservation.ID)

	sharedConflicts, err := reserve.CheckStagedConflicts(ctx, store,
		[]string{"src/shared/utils.go"}, "ts08-agent-D")
	if err != nil {
		return fail(ts08ID, fmt.Sprintf("check shared conflicts: %v", err), details...)
	}
	if len(sharedConflicts) > 0 {
		return fail(ts08ID, "non-exclusive lease should not block other agents", details...)
	}
	details = append(details, "shared lease does not block (AC#4)")

	// 5. Release clears the block
	if err := reserve.ReleaseReservation(ctx, store, result.Reservation.ID); err != nil {
		return fail(ts08ID, fmt.Sprintf("release: %v", err), details...)
	}
	postRelease, err := reserve.CheckStagedConflicts(ctx, store,
		[]string{"src/cmd/issues/create.go"}, "ts08-agent-B")
	if err != nil {
		return fail(ts08ID, fmt.Sprintf("check after release: %v", err), details...)
	}
	if len(postRelease) > 0 {
		return fail(ts08ID, "released lease should not block", details...)
	}
	details = append(details, "released lease allows commits")

	return pass(ts08ID, details...)
}

// Ensure time is used
var _ = time.Now
