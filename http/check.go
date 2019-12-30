package http

import (
	"crypto/tls"
	"fmt"
	"github.com/flanksource/canary-checker/pkg"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func Check(check pkg.HTTPCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult
	for _, endpoint := range check.Endpoints {
		rcOK, contentOK, timeOK, sslOK := false, false, false, false
		lookupResult, err := DNSLookup(endpoint)
		if err != nil {
			log.Printf("Failed to resolve DNS for %s", endpoint)
			return []*pkg.CheckResult{{
				Pass:    false,
				Invalid: true,
				Metrics: []pkg.Metric{},
			}}

		}
		for _, urlObj := range lookupResult {
			checkResults, err := checkHTTP(urlObj)
			if err == nil {
				for _, rc := range check.ResponseCodes {
					if rc == checkResults.ResponseCode {
						rcOK = true
					}
				}
				if check.ResponseContent == checkResults.Content {
					contentOK = true
				}
				if check.ThresholdMillis >= int(checkResults.ResponseTime) {
					timeOK = true
				}
				if check.MaxSSLExpiry <= checkResults.SSLExpiry {
					sslOK = true
				}
				pass := rcOK && contentOK && timeOK && sslOK
				m := []pkg.Metric{
					{Name: "response_time", Value: checkResults.ResponseTime},
					{Name: "response_code", Value: checkResults.ResponseCode},
					{Name: "ssl_certificate_expiry", Value: checkResults.SSLExpiry},
				}
				checkResult := &pkg.CheckResult{
					Pass:    pass,
					Invalid: false,
					Metrics: m,
				}
				result = append(result, checkResult)
			} else {
				checkResult := &pkg.CheckResult{
					Pass:    false,
					Invalid: true,
					Metrics: []pkg.Metric{},
				}
				result = append(result, checkResult)
			}
		}
	}
	return result
}

func checkHTTP(urlObj pkg.URL) (*pkg.HTTPCheckResult, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	var exp time.Time
	start := time.Now()
	var urlString string
	if urlObj.Port > 0 {
		urlString = fmt.Sprintf("%s://%s%s", urlObj.Scheme, net.JoinHostPort(urlObj.IP, strconv.Itoa(urlObj.Port)), urlObj.Path)
	} else {
		urlString = fmt.Sprintf("%s://%s%s", urlObj.Scheme, urlObj.IP, urlObj.Path)
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return nil, err
	}
	req.Host = urlObj.Host
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
	checkResult := pkg.HTTPCheckResult{
		Endpoint:     urlObj.Host,
		Record:       urlObj.IP,
		ResponseCode: resp.StatusCode,
		SSLExpiry:    sslExpiryDaysRounded,
		Content:      content,
		ResponseTime: elapsed.Milliseconds(),
	}
	return &checkResult, nil
}

func DNSLookup(endpoint string) ([]pkg.URL, error) {
	var result []pkg.URL
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	ips, err := net.LookupIP(parsedURL.Hostname())
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		port, _ := strconv.Atoi(parsedURL.Port())
		urlObj := pkg.URL{
			IP:     ip.String(),
			Port:   port,
			Host:   parsedURL.Hostname(),
			Scheme: parsedURL.Scheme,
			Path:   parsedURL.Path,
		}
		result = append(result, urlObj)
	}

	return result, nil

}
