package monitoring

import "time"

// Monitoring defines the interface for monitoring provider
type Monitoring interface {
	// GetRetentionPolicies returns a list of retention policies
	GetRetentionPolicies() ([]RetentionPolicy, error)
	// UpdateRetentionPolicy updates a retention policy
	UpdateRetentionPolicy(RetentionPolicy) error
}

// RetentionPolicy represents a single retention policy
type RetentionPolicy struct {
	// Name is the policy name
	Name string `json:"name"`
	// Duration is the policy duration
	Duration time.Duration `json:"duration"`
}
