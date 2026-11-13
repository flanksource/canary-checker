package checks

import (
	gocontext "context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutyCtx "github.com/flanksource/duty/context"
)

type blockingDriver struct{}

func (blockingDriver) Open(string) (driver.Conn, error) { return blockingConn{}, nil }

type blockingConn struct{}

func (blockingConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (blockingConn) Close() error                        { return nil }
func (blockingConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (blockingConn) QueryContext(ctx gocontext.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestQuerySQLAppliesTimeout(t *testing.T) {
	sql.Register("blocking-test", blockingDriver{})

	check := &v1.SQLCheck{Timeout: 1}
	timeout := check.GetQueryTimeout()

	ctx := context.New(dutyCtx.New(), v1.Canary{})
	done := make(chan error, 1)
	go func() {
		_, err := querySQL(ctx, "blocking-test", "", "SELECT 1", timeout)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a timeout error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("query timeout of 1s was not applied")
	}
}
