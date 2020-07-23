package mg

var shutdownHooks []func()

// AddShutdownHook adds a hook to mages shutdown logic. Mage when shutting down will executed the hooks
// allowing for any clean up.
func AddShutdownHook(f func()) {
	shutdownHooks = append(shutdownHooks, f)
}

// RunShutdownHooks is called by mage to execute any provided shutdown hooks.
func RunShutdownHooks() {
	for _, f := range shutdownHooks {
		f()
	}
}
