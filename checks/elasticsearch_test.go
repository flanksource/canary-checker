package checks

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	checkctx "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutyCtx "github.com/flanksource/duty/context"
)

// trackElasticsearchConns counts client connections opened by the Elasticsearch
// client. The client clones http.DefaultTransport, so DialContext installed
// here is the one that places the call. json.Decoder stops after one value;
// bytes after that value stay unread, so the connection is released only when
// the response body is closed.
func trackElasticsearchConns(t *testing.T) (dials, open *atomic.Int32) {
	t.Helper()
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatalf("http.DefaultTransport is %T, want *http.Transport", http.DefaultTransport)
	}
	dials = &atomic.Int32{}
	open = &atomic.Int32{}
	transport := base.Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		dials.Add(1)
		open.Add(1)
		return &onceCloseConn{Conn: c, open: open}, nil
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	return dials, open
}

type onceCloseConn struct {
	net.Conn
	once sync.Once
	open *atomic.Int32
}

func (c *onceCloseConn) Close() error {
	c.once.Do(func() { c.open.Add(-1) })
	return c.Conn.Close()
}

func elasticsearchTestContext() *checkctx.Context {
	return &checkctx.Context{
		Context:   dutyCtx.New(),
		Namespace: "default",
		Canary:    v1.Canary{},
	}
}

func waitConnsClosed(t *testing.T, open *atomic.Int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if open.Load() == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("response body was not closed: %d connection(s) still open", open.Load())
}

func TestElasticsearchCheckClosesResponseBody(t *testing.T) {
	// Trailing padding is larger than the JSON decoder buffer so Decode does
	// not read the body to EOF. The TCP connection then stays open until Close.
	pad := strings.Repeat(" ", 8<<10)

	tests := []struct {
		name    string
		status  int
		body    string
		header  string
		results int
		wantErr string
		wantOK  bool
	}{
		{
			name:    "rejected query",
			status:  http.StatusBadRequest,
			body:    `{"error":{"type":"parsing_exception","reason":"unknown query"},"status":400}` + pad,
			wantErr: "parsing_exception",
		},
		{
			name:    "rejected query with unreadable body",
			status:  http.StatusBadRequest,
			body:    `not-json` + pad,
			wantErr: "error parsing the response body",
		},
		{
			name:    "successful query",
			status:  http.StatusOK,
			body:    `{"hits":{"total":{"value":1}}}` + pad,
			header:  "Elasticsearch",
			results: 1,
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Connection", "close")
				w.Header().Set("Content-Type", "application/json")
				if tt.header != "" {
					w.Header().Set("X-Elastic-Product", tt.header)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			dials, open := trackElasticsearchConns(t)
			check := v1.ElasticsearchCheck{
				Description: v1.Description{Name: "elasticsearch"},
				Connection:  v1.Connection{URL: server.URL},
				Index:       "logs",
				Query:       `{"query":{"match_all":{}}}`,
				Results:     tt.results,
			}

			results := (&ElasticsearchChecker{}).Check(elasticsearchTestContext(), check)
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if dials.Load() == 0 {
				t.Fatalf("client did not connect; result error=%q", results[0].Error)
			}
			if tt.wantOK {
				if !results[0].Pass {
					t.Fatalf("expected the check to pass, got %q", results[0].Error)
				}
			} else if results[0].Pass || !strings.Contains(results[0].Error, tt.wantErr) {
				t.Fatalf("expected failure containing %q, got pass=%v error=%q", tt.wantErr, results[0].Pass, results[0].Error)
			}
			waitConnsClosed(t, open)
		})
	}
}

func TestElasticsearchTransportErrorDoesNotPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	check := v1.ElasticsearchCheck{
		Description: v1.Description{Name: "elasticsearch"},
		Connection:  v1.Connection{URL: server.URL},
		Index:       "logs",
		Query:       `{"query":{"match_all":{}}}`,
	}

	results := (&ElasticsearchChecker{}).Check(elasticsearchTestContext(), check)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Pass {
		t.Fatal("expected the check to fail when Elasticsearch is unreachable")
	}
	if results[0].Error == "" {
		t.Fatal("expected an error when Elasticsearch is unreachable")
	}
}
