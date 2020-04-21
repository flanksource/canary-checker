package checks

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/canary-checker/pkg"
	ldap "github.com/go-ldap/ldap/v3"
)

var (
	ldapLookupRecordCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_ldap_record_count",
			Help: "LDAP Record Count",
		},
		[]string{"endpoint", "bindDN"},
	)
)

func init() {
	prometheus.MustRegister(ldapLookupRecordCount)
}

type LdapChecker struct{}

// Type: returns checker type
func (c *LdapChecker) Type() string {
	return "ldap"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *LdapChecker) Run(config pkg.Config, results chan *pkg.CheckResult) {
	for _, conf := range config.LDAP {
		for _, result := range c.Check(conf.LDAPCheck) {
			results <- result
		}
	}
}

// CheckConfig : Check every ldap entry for lookup and auth
// Returns check result and metrics
func (c *LdapChecker) Check(check pkg.LDAPCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult

	endpoint := fmt.Sprintf("%s/%s/%s", check.Host, check.BindDN, check.UserSearch)

	ld, err := ldap.DialURL(check.Host, ldap.DialWithTLSConfig(&tls.Config{
		InsecureSkipVerify: check.SkipTLSVerify,
	}))
	if err != nil {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Message:  fmt.Sprintf("Failed to connect to LDAP url %s: %v", check.Host, err),
			Endpoint: endpoint,
		})
		return result
	}

	if err := ld.Bind(check.Username, check.Password); err != nil {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Message:  fmt.Sprintf("Failed to bind using credentials given to LDAP url %s: %v", check.Host, err),
			Endpoint: endpoint,
		})
		return result
	}

	req := &ldap.SearchRequest{
		BaseDN: check.BindDN,
		Filter: check.UserSearch,
	}
	timer := NewTimer()
	res, err := ld.Search(req)

	if err != nil {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Message:  fmt.Sprintf("Failed to search to LDAP url %s: %v", check.Host, err),
			Endpoint: endpoint,
		})
		return result
	}

	ldapLookupRecordCount.WithLabelValues(check.Host, check.BindDN).Set(float64(len(res.Entries)))

	result = append(result, &pkg.CheckResult{
		Pass:     true,
		Endpoint: endpoint,
		Message:  fmt.Sprintf("LDAP search %s for host %s DN %s successful", check.UserSearch, check.Host, check.BindDN),
		Duration: int64(timer.Elapsed()),
	})

	return result
}
