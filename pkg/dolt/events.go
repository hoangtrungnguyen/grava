package dolt

// Audit event type constants. Always use these constants — never raw string literals.
const (
	EventCreate        = "create"
	EventUpdate        = "update"
	EventClaim         = "claim"
	EventRelease       = "release"
	EventDrop          = "drop"
	EventClear         = "clear"
	EventStart         = "start"
	EventStop          = "stop"
	EventClose         = "close"
	EventImport        = "import"
	EventExport        = "export"
	EventUndo          = "undo"
	EventLabel         = "label"
	EventComment       = "comment"
	EventAssign        = "assign"
	EventSubtask       = "subtask"
	EventReserve       = "reserve"
	EventDependencyAdd = "dependency_add"
	EventWispWrite     = "wisp_write"
)
