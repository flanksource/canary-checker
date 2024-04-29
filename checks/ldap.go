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
	var err error
	results = append(results, result)

	connection, err := ctx.GetConnection(check.Connection)
	if err != nil {
		return results.Errorf("failed to get connection: %v", err)
	}

	if connection.URL == "" {
		return results.Invalidf("Must specify a connection or URL")
	}

	ld, err := ldap.DialURL(connection.URL, ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: check.SkipTLSVerify}))
	if err != nil {
		return results.Failf("Failed to connect %v", err)
	}

	if err := ld.Bind(connection.Username, connection.Password); err != nil {
		return results.Failf("Failed to bind using %s %v", connection.Username, err)
	}

	req := &ldap.SearchRequest{
		Scope:  ldap.ScopeWholeSubtree,
		BaseDN: check.BindDN,
		Filter: check.UserSearch,
	}
	res, err := ld.Search(req)
	if err != nil {
		return results.Errorf("Failed to search host %v error: %v", connection.URL, err)
	}

	if len(res.Entries) == 0 {
		return results.Failf("no results returned")
	}

	return results
}
