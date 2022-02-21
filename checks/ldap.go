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
func (c *LdapChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.LDAP {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// CheckConfig : Check every ldap entry for lookup and auth
// Returns check result and metrics
func (c *LdapChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.LDAPCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	ld, err := ldap.DialURL(check.Host, ldap.DialWithTLSConfig(&tls.Config{
		InsecureSkipVerify: check.SkipTLSVerify,
	}))
	if err != nil {
		return results.Failf("Failed to connect %v", err)
	}
	namespace := ctx.Canary.Namespace
	auth, err := GetAuthValues(check.Auth, ctx.Kommons, namespace)
	if err != nil {
		return results.Failf("failed to fetch auth details: %v", err)
	}
	if err := ld.Bind(auth.Username.Value, auth.Password.Value); err != nil {
		return results.Failf("Failed to bind using %s %v", auth.Username.Value, err)
	}

	req := &ldap.SearchRequest{
		Scope:  ldap.ScopeWholeSubtree,
		BaseDN: check.BindDN,
		Filter: check.UserSearch,
	}
	res, err := ld.Search(req)

	if err != nil {
		return results.Failf("Failed to search host %v error: %v", check.Host, err)
	}

	if len(res.Entries) == 0 {
		return results.Failf("no results returned")
	}

	return results
}
