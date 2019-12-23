package pkg

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// URL information
type URL struct {
	ip     string
	port   int
	host   string
	scheme string
	path   string
}

// CheckConfig : Check every record of DNS name agaist config information
// Returns check result and metrics
func CheckConfig(args map[string]interface{}) []*CheckResult {
	http, ok := args["http"]
	var result []*CheckResult
	if ok {
		for _, endpoint := range http.(HTTP).Endpoints {
			rcOK, contentOK, timeOK, sslOK := false, false, false, false
			lookupResult := dnsLookup(endpoint)
			for _, urlObj := range lookupResult {
				checkResults, err := checkHTTP(urlObj)
				if err == nil {
					for _, rc := range http.(HTTP).ResponseCodes {
						if rc == checkResults.(HTTPCheckResult).ResponseCode {
							rcOK = true
						}
					}
					if http.(HTTP).ResponseContent == checkResults.(HTTPCheckResult).Content {
						contentOK = true
					}
					if http.(HTTP).ThresholdMillis >= int(checkResults.(HTTPCheckResult).ResponseTime) {
						timeOK = true
					}
					if http.(HTTP).MaxSSLExpiry <= checkResults.(HTTPCheckResult).SSLExpiry {
						sslOK = true
					}
					pass := rcOK && contentOK && timeOK && sslOK
					m := []Metric{
						Metric{Name: "response_time", Value: checkResults.(HTTPCheckResult).ResponseTime},
						Metric{Name: "response_code", Value: checkResults.(HTTPCheckResult).ResponseCode},
						Metric{Name: "ssl_certificate_expiry", Value: checkResults.(HTTPCheckResult).SSLExpiry},
					}
					checkResult := &CheckResult{
						Pass:    pass,
						Invalid: false,
						Metrics: m,
					}
					result = append(result, checkResult)
				} else {
					checkResult := &CheckResult{
						Pass:    false,
						Invalid: true,
						Metrics: []Metric{},
					}
					result = append(result, checkResult)
				}
			}
		}
	}
	return result
}

func checkHTTP(urlObj URL) (interface{}, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	var exp time.Time
	start := time.Now()
	var url string
	if urlObj.port > 0 {
		url = fmt.Sprintf("%s://%s%s", urlObj.scheme, net.JoinHostPort(urlObj.ip, strconv.Itoa(urlObj.port)), urlObj.path)
	} else {
		url = fmt.Sprintf("%s://%s%s", urlObj.scheme, urlObj.ip, urlObj.path)
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Host = urlObj.host
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.TLS != nil {
		certificates := resp.TLS.PeerCertificates
		if len(certificates) > 0 {
			exp = certificates[0].NotAfter
		}
	}
	if err != nil {
		return nil, err
	}
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	content := string(res)
	sslExpireDays := int(exp.Sub(start).Hours() / 24.0)
	var sslExpiryDaysRounded int
	if sslExpireDays <= 0 {
		sslExpiryDaysRounded = 0
	} else {
		sslExpiryDaysRounded = sslExpireDays
	}

	defer resp.Body.Close()
	elapsed := time.Since(start)
	checkResult := HTTPCheckResult{
		Endpoint:     urlObj.host,
		Record:       urlObj.ip,
		ResponseCode: resp.StatusCode,
		SSLExpiry:    sslExpiryDaysRounded,
		Content:      content,
		ResponseTime: elapsed.Milliseconds(),
	}
	return checkResult, nil
}

func dnsLookup(endpoint string) []URL {
	var result []URL
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		log.Fatal(err)
	}
	ips, err := net.LookupIP(parsedURL.Hostname())
	if err != nil {
		log.Fatal(err)
	}
	for _, ip := range ips {
		port, _ := strconv.Atoi(parsedURL.Port())
		urlObj := URL{
			ip:     ip.String(),
			port:   port,
			host:   parsedURL.Hostname(),
			scheme: parsedURL.Scheme,
			path:   parsedURL.Path,
		}
		result = append(result, urlObj)
	}

	return result

}
