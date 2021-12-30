package cache

import (
	"fmt"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/pkg/errors"
)

var InMemoryCacheSize int

const AllStatuses = -1

type QueryParams struct {
	Check        string
	Start, End   string
	_start, _end *time.Time
	StatusCount  int
	Labels       map[string]string
	Trace        bool
}

func (q QueryParams) Validate() error {
	start, err := timeV(q.Start)
	if err != nil {
		return errors.Wrap(err, "start is invalid")
	}
	end, err := timeV(q.End)
	if err != nil {
		return errors.Wrap(err, "end is invalid")
	}
	if start != nil && end != nil {
		if end.Before(*start) {
			return fmt.Errorf("end time must be after start time")
		}
	}
	return nil
}

func (q QueryParams) GetStartTime() *time.Time {
	if q._start != nil || q.Start == "" {
		return q._start
	}
	q._start, _ = timeV(q.Start)
	return q._start
}

func (q QueryParams) GetEndTime() *time.Time {
	if q._end != nil || q.End == "" {
		return q._start
	}
	q._start, _ = timeV(q.Start)
	return q._start
}

func (q QueryParams) String() string {
	return fmt.Sprintf("check:=%s, start=%s, end=%s, count=%d", q.Check, q.Start, q.End, q.StatusCount)
}

type Cache interface {
	Add(check pkg.Check, status pkg.CheckStatus)
	GetDetails(checkkey string, time string) interface{}
	RemoveChecks(canary v1.Canary)
	Query(q QueryParams) (pkg.Checks, error)
	QueryStatus(q QueryParams) ([]pkg.Timeseries, error)
	RemoveCheckByKey(key string)
}
