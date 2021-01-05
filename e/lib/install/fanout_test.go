package install

import (
	"time"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/testhelpers"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"

	"gopkg.in/check.v1"
)

type FanoutSuite struct {
	operator *fanoutOperator
}

var _ = check.Suite(&FanoutSuite{})

func (s *FanoutSuite) SetUpSuite(c *check.C) {
	services := opsservice.SetupTestServices(c)
	operator := &testOperator{Operator: testhelpers.WrapOperator(services.Operator)}
	// just use the same operator twice for simplicity
	s.operator = NewFanoutOperator(operator, operator)
}

func (s *FanoutSuite) TestCreateProgressEntry(c *check.C) {
	entry := ossops.ProgressEntry{Created: time.Now().UTC()}
	err := s.operator.CreateProgressEntry(ossops.SiteOperationKey{}, entry)
	c.Assert(err, check.IsNil)
	// since we've used the same operator, now there should be two entries
	savedEntries := s.operator.Operator.(*testOperator).progressEntries
	c.Assert(savedEntries, check.DeepEquals, []ossops.ProgressEntry{entry, entry})
}

func (s *FanoutSuite) TestCreateLogEntry(c *check.C) {
	entry := ossops.LogEntry{Created: time.Now().UTC()}
	err := s.operator.CreateLogEntry(ossops.SiteOperationKey{}, entry)
	c.Assert(err, check.IsNil)
	// since we've used the same operator, now there should be two entries
	savedEntries := s.operator.Operator.(*testOperator).logEntries
	c.Assert(savedEntries, check.DeepEquals, []ossops.LogEntry{entry, entry})
}

// testOperator simplifies the testing of the fanout operator by keeping
// progress and log entries in memory
type testOperator struct {
	ops.Operator
	progressEntries []ossops.ProgressEntry
	logEntries      []ossops.LogEntry
}

func (o *testOperator) CreateProgressEntry(key ossops.SiteOperationKey, entry ossops.ProgressEntry) error {
	o.progressEntries = append(o.progressEntries, entry)
	return nil
}

func (o *testOperator) CreateLogEntry(key ossops.SiteOperationKey, entry ossops.LogEntry) error {
	o.logEntries = append(o.logEntries, entry)
	return nil
}
