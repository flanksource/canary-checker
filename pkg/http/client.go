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

	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	httpntlm "github.com/vadimi/go-http-ntlm/v2"
)

type HttpRequest struct {
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
	tr                      *http.Transport
	traceHeaders, traceBody bool
}

func NewRequest(endpoint string) *HttpRequest {
	url, _ := url.Parse(endpoint)
	return &HttpRequest{
		host:    url.Host,
		URL:     url,
		start:   time.Now(),
		headers: make(map[string]string),
	}
}

func (h *HttpRequest) Method(method string) *HttpRequest {
	h.method = method
	return h
}

func (h *HttpRequest) UseHost(host string) *HttpRequest {
	h.host = host
	return h
}

func (h *HttpRequest) Debug(debug bool) *HttpRequest {
	h.traceHeaders = debug
	return h
}

func (h *HttpRequest) Trace(trace bool) *HttpRequest {
	h.Debug(trace)
	h.traceBody = trace
	return h
}

func (h *HttpRequest) Host(host string) *HttpRequest {
	h.host = host
	return h
}

func (h *HttpRequest) Header(header, value string) *HttpRequest {
	h.headers[header] = value
	return h
}

func (h *HttpRequest) Auth(username, password string) *HttpRequest {
	h.Username = username
	h.Password = password
	return h
}

func (h *HttpRequest) NTLM(ntlm bool) *HttpRequest {
	h.ntlm = ntlm
	return h
}

func (h *HttpRequest) Insecure(skip bool) *HttpRequest {
	h.insecure = skip
	return h
}

func (h *HttpRequest) Headers(headers map[string]string) *HttpRequest {
	h.headers = headers
	return h
}

func (h *HttpRequest) getHttpClient() *http.Client {
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

func (h *HttpRequest) GetRequestLine() string {
	s := fmt.Sprintf("%s %s", h.method, h.URL)
	if h.host != h.URL.Hostname() {
		s += fmt.Sprintf(" (%s)", h.host)
	}
	return s
}

func (h *HttpRequest) GetString() string {
	s := h.GetRequestLine()
	if h.traceHeaders {
		s += "\n"
		for k, values := range h.headers {
			s += k + ": " + values + "\n"
		}
	}
	return s
}

func (h *HttpResponse) IsOK(responseCodes ...int) bool {
	code := h.Response.StatusCode
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

func (h *HttpRequest) Do(body string) *HttpResponse {
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
	if logger.IsTraceEnabled() {
		logger.Tracef(h.GetString())
	}

	for header, field := range h.headers {
		req.Header.Add(header, field)
	}
	if h.Username != "" && h.Password != "" {
		req.SetBasicAuth(h.Username, h.Password)
	}

	resp, err := h.getHttpClient().Do(req)
	r := NewHttpResponse(h, resp).SetError(err)

	if logger.IsTraceEnabled() {
		logger.Tracef(r.String())
	}
	return r

}

func NewHttpResponse(req *HttpRequest, resp *http.Response) *HttpResponse {
	headers := make(map[string]string)
	if resp != nil {
		for header, values := range resp.Header {
			headers[header] = strings.Join(values, " ")
		}
	}
	return &HttpResponse{
		Request:  req,
		Headers:  headers,
		Response: resp,
		Elapsed:  time.Since(req.start),
	}
}

type HttpResponse struct {
	Request *HttpRequest
	Headers map[string]string
	*http.Response
	Elapsed time.Duration
	Error   error
	body    string
}

func (h *HttpResponse) SetError(err error) *HttpResponse {
	h.Error = err
	return h
}

func (h *HttpResponse) Start(start time.Time) *HttpResponse {
	h.Elapsed = time.Since(start)
	return h
}

func (h *HttpResponse) String() string {
	s := fmt.Sprintf("%s [%s] %d", h.Request.GetRequestLine(), utils.Age(h.Elapsed), h.StatusCode)
	if h.Request.traceHeaders {
		s += "\n"
		for k, values := range h.Header {
			s += k + ": " + strings.Join(values, " ") + "\n"
		}
	}
	if h.Request.traceBody {
		body, _ := h.AsString()
		s += "\n" + body
	}
	return s
}

func (h *HttpResponse) GetSSLAge() *time.Duration {
	if h.TLS == nil {
		return nil
	}
	certificates := h.TLS.PeerCertificates
	if len(certificates) == 0 {
		return nil
	}

	age := time.Until(certificates[0].NotAfter)
	return &age
}

func (h *HttpResponse) IsJSON() bool {
	return strings.HasPrefix(h.Headers["Content-Type"], "application/json")
}

func (h *HttpResponse) AsJSON() (*JSON, error) {
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

func (h *HttpResponse) AsString() (string, error) {
	if h.body != "" {
		return h.body, nil
	}
	res, err := ioutil.ReadAll(h.Body)
	if err != nil {
		return "", err
	}
	h.body = string(res)
	return h.body, nil
}
