package validation

import (
	"testing"

	"golang.org/x/net/context"

	. "gopkg.in/check.v1"
)

func TestValidation(t *testing.T) { TestingT(t) }

type ValidationSuite struct{}

var _ = Suite(&ValidationSuite{})

func (r *ValidationSuite) TestAddsDependencies(c *C) {
	validator := fakeValidator{}
	client := &clientContext{}
	probes := Probes{
		{Action{IAM, "AddRoleToInstanceProfile"}, validateAddRoleToInstanceProfile},
	}
	actions, err := validateWithContext(client, probes, validator, context.TODO())
	c.Assert(err, IsNil)
	c.Assert(actions, DeepEquals, Actions{
		{IAM, "AddRoleToInstanceProfile"},
		{IAM, "PassRole"},
	})
}

func (r *ValidationSuite) TestEmptyStatementAlwaysValidates(c *C) {
	validator := fakeValidator{}
	client := &clientContext{}
	var probes Probes
	actions, err := validateWithContext(client, probes, validator, context.TODO())
	c.Assert(err, IsNil)
	c.Assert(actions, IsNil)
}

type fakeValidator struct{}

func (r fakeValidator) Do(client *clientContext, probe ResourceProbe) (bool, error) {
	return false, nil
}
