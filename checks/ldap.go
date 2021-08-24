package checks

import (
	"crypto/tls"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	ldap "github.com/go-ldap/ldap/v3"
)

type LdapChecker struct {
}

// Type: returns checker type
func (c *LdapChecker) Type() string {
	return "ldap"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *LdapChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.LDAP {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

// CheckConfig : Check every ldap entry for lookup and auth
// Returns check result and metrics
func (c *LdapChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.LDAPCheck)
	ld, err := ldap.DialURL(check.Host, ldap.DialWithTLSConfig(&tls.Config{
		InsecureSkipVerify: check.SkipTLSVerify,
	}))
	if err != nil {
		return Failf(check, "Failed to connect %v", err)
	}
	namespace := ctx.Canary.Namespace
	auth, err := GetAuthValues(check.Auth, ctx.Kommons, namespace)
	if err != nil {
		return Failf(check, "failed to fetch auth details: %v", err)
	}
	if err := ld.Bind(auth.Username.Value, auth.Password.Value); err != nil {
		return Failf(check, "Failed to bind using %s %v", auth.Username.Value, err)
	}

	req := &ldap.SearchRequest{
		Scope:  ldap.ScopeWholeSubtree,
		BaseDN: check.BindDN,
		Filter: check.UserSearch,
	}

	timer := NewTimer()
	res, err := ld.Search(req)

	if err != nil {
		return Failf(check, "Failed to search host %v error: %v", check.Host, err)
	}

	if len(res.Entries) == 0 {
		return Failf(check, "no results returned")
	}

	return &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Duration: int64(timer.Elapsed()),
	}
}
