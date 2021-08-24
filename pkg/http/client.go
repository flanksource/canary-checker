package http

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PaesslerAG/jsonpath"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	httpntlm "github.com/vadimi/go-http-ntlm/v2"
)

type HTTPRequest struct {
	Username                string
	Password                string
	method                  string
	host                    string
	Port                    int
	URL                     *url.URL
	start                   time.Time
	headers                 map[string]string
	insecure                bool
	ntlm                    bool
	tr                      *http.Transport //nolint:structcheck,unused
	traceHeaders, traceBody bool
}

func NewRequest(endpoint string) *HTTPRequest {
	url, _ := url.Parse(endpoint)
	return &HTTPRequest{
		host:    url.Host,
		URL:     url,
		start:   time.Now(),
		headers: make(map[string]string),
	}
}

func (h *HTTPRequest) Method(method string) *HTTPRequest {
	h.method = method
	return h
}

func (h *HTTPRequest) UseHost(host string) *HTTPRequest {
	h.host = host
	return h
}

func (h *HTTPRequest) Debug(debug bool) *HTTPRequest {
	h.traceHeaders = debug
	return h
}

func (h *HTTPRequest) Trace(trace bool) *HTTPRequest {
	h.Debug(trace)
	h.traceBody = trace
	return h
}

func (h *HTTPRequest) Host(host string) *HTTPRequest {
	h.host = host
	return h
}

func (h *HTTPRequest) Header(header, value string) *HTTPRequest {
	h.headers[header] = value
	return h
}

func (h *HTTPRequest) Auth(username, password string) *HTTPRequest {
	h.Username = username
	h.Password = password
	return h
}

func (h *HTTPRequest) NTLM(ntlm bool) *HTTPRequest {
	h.ntlm = ntlm
	return h
}

func (h *HTTPRequest) Insecure(skip bool) *HTTPRequest {
	h.insecure = skip
	return h
}

func (h *HTTPRequest) Headers(headers map[string]string) *HTTPRequest {
	h.headers = headers
	return h
}

func (h *HTTPRequest) getHTTPClient() *http.Client {
	var transport http.RoundTripper
	transport = &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         h.host,
		},
	}

	if h.ntlm {
		parts := strings.Split(h.Username, "@")
		domain := ""
		if len(parts) > 1 {
			domain = parts[1]
		}

		transport = &httpntlm.NtlmTransport{
			Domain:       domain,
			User:         parts[0],
			Password:     h.Password,
			RoundTripper: transport,
		}
	}

	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (h *HTTPRequest) GetRequestLine() string {
	s := fmt.Sprintf("%s %s", h.method, h.URL)
	if h.host != h.URL.Hostname() {
		s += fmt.Sprintf(" (%s)", h.host)
	}
	return s
}

func (h *HTTPRequest) GetString() string {
	s := h.GetRequestLine()
	if h.traceHeaders {
		s += "\n"
		for k, values := range h.headers {
			s += k + ": " + values + "\n"
		}
	}
	return s
}

func (h *HTTPResponse) IsOK(responseCodes ...int) bool {
	code := h.GetStatusCode()
	if h.Error != nil {
		return false
	}
	if len(responseCodes) == 0 {
		return code >= 200 && code < 299
	}
	for _, valid := range responseCodes {
		if code == valid {
			return true
		}
	}
	return false
}

func (h *HTTPRequest) Do(body string) *HTTPResponse {
	if h.host != h.URL.Hostname() {
		// If specified, replace the hostname in the URL, with the actual host/IP connect to
		// and move the Virtual Hostname to a Header
		h.headers["Host"] = h.URL.Hostname()
		h.URL.Host = h.host
		if h.URL.Port() == "" {
			h.URL.Host = h.host
		} else {
			h.URL.Host = h.host + ":" + h.URL.Port()
		}
	}

	req, err := http.NewRequest(h.method, h.URL.String(), strings.NewReader(body))
	if err != nil {
		return nil
	}
	if logger.IsTraceEnabled() {
		logger.Tracef(h.GetString())
	}

	for header, field := range h.headers {
		req.Header.Add(header, field)
	}
	if h.Username != "" && h.Password != "" {
		req.SetBasicAuth(h.Username, h.Password)
	}

	resp, err := h.getHTTPClient().Do(req)
	r := NewHTTPResponse(h, resp).SetError(err)

	if logger.IsTraceEnabled() {
		logger.Tracef(r.String())
	}
	return r
}

func NewHTTPResponse(req *HTTPRequest, resp *http.Response) *HTTPResponse {
	headers := make(map[string]string)
	if resp != nil {
		for header, values := range resp.Header {
			headers[header] = strings.Join(values, " ")
		}
	}
	return &HTTPResponse{
		Request:  req,
		Headers:  headers,
		Response: resp,
		Elapsed:  time.Since(req.start),
	}
}

type HTTPResponse struct {
	Request  *HTTPRequest
	Headers  map[string]string
	Response *http.Response
	Elapsed  time.Duration
	Error    error
	body     string
}

// GetStatusCode returns the HTTP Status Code or -1 if there was a network error
func (h *HTTPResponse) GetStatusCode() int {
	if h.Response == nil {
		return -1
	}
	return h.Response.StatusCode

}

func getMapFromHeader(header http.Header) map[string]string {
	m := make(map[string]string)
	for k, v := range header {
		m[k] = strings.Join(v, " ")
	}
	return m

}
func (h *HTTPResponse) GetHeaders() map[string]string {
	if h.Response == nil {
		return make(map[string]string)
	}
	return getMapFromHeader(h.Response.Header)
}

func (h *HTTPResponse) SetError(err error) *HTTPResponse {
	h.Error = err
	return h
}

func (h *HTTPResponse) Start(start time.Time) *HTTPResponse {
	h.Elapsed = time.Since(start)
	return h
}

func (h *HTTPResponse) String() string {
	s := fmt.Sprintf("%s [%s] %d", h.Request.GetRequestLine(), utils.Age(h.Elapsed), h.GetStatusCode())
	if h.Request.traceHeaders {
		s += "\n"
		for k, values := range h.Response.Header {
			s += k + ": " + strings.Join(values, " ") + "\n"
		}
	}
	if h.Request.traceBody {
		body, _ := h.AsString()
		s += "\n" + body
	}
	return s
}

func (h *HTTPResponse) GetSSLAge() *time.Duration {
	if h.Response == nil {
		return nil
	}
	if h.Response.TLS == nil {
		return nil
	}
	certificates := h.Response.TLS.PeerCertificates
	if len(certificates) == 0 {
		return nil
	}

	age := time.Until(certificates[0].NotAfter)
	return &age
}

func (h *HTTPResponse) IsJSON() bool {
	return strings.HasPrefix(h.Headers["Content-Type"], "application/json")
}

func (h *HTTPResponse) AsJSON() (*JSON, error) {
	if h.Response == nil {
		return nil, fmt.Errorf("request did not complete with a body")
	}

	var jsonContent interface{}
	s, err := h.AsString()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(s), &jsonContent); err != nil {
		return nil, err
	}
	return &JSON{Value: jsonContent}, nil
}

func (h *HTTPResponse) AsString() (string, error) {
	if h.Response == nil {
		return "", fmt.Errorf("request did not complete with a body")
	}
	if h.body != "" {
		return h.body, nil
	}
	res, err := ioutil.ReadAll(h.Response.Body)
	defer h.Response.Body.Close() //nolint
	if err != nil {
		return "", err
	}
	h.body = string(res)
	return h.body, nil
}

func (h *HTTPResponse) CheckJSONContent(jsonContent interface{}, jsonCheck v1.JSONCheck) error {
	jsonResult, err := jsonpath.Get(jsonCheck.Path, jsonContent)
	if err != nil {
		logger.Errorf("Error checking JSON content: %s", err)
		return err
	}
	switch s := jsonResult.(type) {
	case string:
		if s != jsonCheck.Value {
			return fmt.Errorf("%v not equal to %v", s, jsonCheck.Value)
		}
	case fmt.Stringer:
		if s.String() != jsonCheck.Value {
			return fmt.Errorf("%v not equal to %v", s.String(), jsonCheck.Value)
		}
	default:
		return fmt.Errorf("json response could not be parsed back to string")
	}
	return nil
}
