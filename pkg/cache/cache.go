package cache

import (
	"fmt"
	"strconv"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

var DefaultCacheCount int

var DefaultWindow string

const AllStatuses = -1

type QueryParams struct {
	Check           string
	Start, End      string
	Window          string
	IncludeMessages bool
	IncludeDetails  bool
	_start, _end    *time.Time
	StatusCount     int
	Labels          map[string]string
	Trace           bool
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

func ParseQuery(c echo.Context) (*QueryParams, error) {
	queryParams := c.Request().URL.Query()
	count := queryParams.Get("count")
	var cacheCount int64
	var err error
	if count != "" {
		cacheCount, err = strconv.ParseInt(count, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("count must be a number: %s", count)
		}
	} else {
		cacheCount = int64(DefaultCacheCount)
	}
	since := queryParams.Get("since")
	if since == "" {
		since = queryParams.Get("start")
	}
	if since == "" {
		since = DefaultWindow
	}
	until := queryParams.Get("until")
	if until == "" {
		until = queryParams.Get("end")
	}
	q := QueryParams{
		Start:           since,
		End:             until,
		Window:          queryParams.Get("window"),
		IncludeMessages: isTrue(queryParams.Get("includeMessages")),
		IncludeDetails:  isTrue(queryParams.Get("includeDetails")),
		Check:           queryParams.Get("check"),
		StatusCount:     int(cacheCount),
		Trace:           isTrue(queryParams.Get("trace")),
	}

	if err := q.Validate(); err != nil {
		return nil, err
	}

	return &q, nil
}

func isTrue(v string) bool {
	return v == "true"
}

type Cache interface {
	Add(check pkg.Check, status ...pkg.CheckStatus)
	GetDetails(checkkey string, time string) interface{}
	RemoveChecks(canary v1.Canary)
	Query(q QueryParams) (pkg.Checks, error)
	QueryStatus(q QueryParams) ([]pkg.Timeseries, error)
	RemoveCheckByKey(key string)
}
