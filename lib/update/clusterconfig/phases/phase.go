package phases

const (
	// InitPhase identifies the initialization phase that resets services
	// back to their original values on rollback
	InitPhase = "init"
	// FiniPhase identifies the finalization phase that commits
	// changes after a successful operation
	FiniPhase = "fini"
)
