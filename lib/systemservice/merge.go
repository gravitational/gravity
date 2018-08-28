package systemservice

// MergeInto applies preset config to existing request if request's field
// is not initialized. Returns result of merge
func MergeInto(req NewPackageServiceRequest, preset NewPackageServiceRequest) NewPackageServiceRequest {
	if req.GravityPath == "" {
		req.GravityPath = preset.GravityPath
	}
	if req.Package.IsEmpty() {
		req.Package = preset.Package
	}
	if req.ConfigPackage.IsEmpty() {
		req.ConfigPackage = preset.ConfigPackage
	}
	if req.StartCommand == "" {
		req.StartCommand = preset.StartCommand
	}
	if req.StartPreCommand == "" {
		req.StartPreCommand = preset.StartPreCommand
	}
	if req.StartPostCommand == "" {
		req.StartPostCommand = preset.StartPostCommand
	}
	if req.StopCommand == "" {
		req.StopCommand = preset.StopCommand
	}
	if req.Timeout == 0 {
		req.Timeout = preset.Timeout
	}
	if req.StopPostCommand == "" {
		req.StopPostCommand = preset.StopPostCommand
	}
	if req.User == "" {
		req.User = preset.User
	}
	if req.Type == "" {
		req.Type = preset.Type
	}
	if req.LimitNoFile == 0 {
		req.LimitNoFile = preset.LimitNoFile
	}
	if req.Type == "" {
		req.Type = preset.Type
	}
	if req.Restart == "" {
		req.Restart = preset.Restart
	}
	if req.KillMode == "" {
		req.KillMode = preset.KillMode
	}
	if len(req.Environment) == 0 {
		req.Environment = preset.Environment
	}
	return req
}
