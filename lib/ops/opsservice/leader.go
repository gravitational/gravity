package opsservice

import "github.com/gravitational/gravity/lib/ops"

// StepDown asks the process to pause its leader election heartbeat so it can
// give up its leadership
func (o *Operator) StepDown(key ops.SiteKey) error {
	o.cfg.Leader.StepDown()
	return nil
}
