package checks

import (
	"crypto/tls"

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
		results <- c.Check(conf.LDAPCheck)
	}
}

// CheckConfig : Check every ldap entry for lookup and auth
// Returns check result and metrics
func (c *LdapChecker) Check(check pkg.LDAPCheck) *pkg.CheckResult {
	ld, err := ldap.DialURL(check.Host, ldap.DialWithTLSConfig(&tls.Config{
		InsecureSkipVerify: check.SkipTLSVerify,
	}))
	if err != nil {
		return Failf(check, "Failed to connect %v", err)
	}

	if err := ld.Bind(check.Username, check.Password); err != nil {
		return Failf(check, "Failed to bind using credentials %v", err)
	}

	req := &ldap.SearchRequest{
		BaseDN: check.BindDN,
		Filter: check.UserSearch,
	}
	timer := NewTimer()
	res, err := ld.Search(req)

	if err != nil {
		return Failf(check, "Failed to search %v", check.Host, err)
	}

	ldapLookupRecordCount.WithLabelValues(check.Host, check.BindDN).Set(float64(len(res.Entries)))

	return &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Duration: int64(timer.Elapsed()),
	}
}
