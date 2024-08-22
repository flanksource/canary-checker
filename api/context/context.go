package context

import (
	"errors"
	"fmt"
	"strings"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	dutyCtx "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/flanksource/gomplate/v3"
)

var DefaultContext dutyCtx.Context

type Context struct {
	Namespace   string
	Canary      v1.Canary
	Environment map[string]interface{}
	cache       map[string]any
	dutyCtx.Context
}

func (ctx *Context) String() string {
	return fmt.Sprintf("%s/%s", ctx.Canary.Namespace, ctx.Canary.Name)
}

func getDomain(username string) string {
	parts := strings.Split(username, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func (ctx *Context) Template(check external.Check, template string) (string, error) {
	env := ctx.Environment

	tpl := gomplate.Template{Template: template}
	if tpl.Functions == nil {
		tpl.Functions = make(map[string]any)
	}
	for k, v := range ctx.GetContextualFunctions() {
		tpl.Functions[k] = v
	}
	out, err := gomplate.RunExpression(env, tpl)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", out), nil
}

func (ctx *Context) CanTemplate() bool {
	return ctx.Canary.Annotations["template"] != "false"
}

func (ctx *Context) GetConnectionTemplate(conn v1.Connection) (*models.Connection, map[string]any, error) {

	var _conn *models.Connection
	var err error

	if _conn, err = ctx.HydrateConnectionByURL(conn.Connection); err != nil {
		return nil, nil, err
	}

	if _conn == nil {
		_conn = &models.Connection{
			URL: conn.URL,
		}
	}

	if conn.URL != "" {
		// override the url specified at the connection level
		_conn.URL = conn.URL
	}

	if _conn.Username == "" || _conn.Password == "" {
		// no username specified at connection level, use the one from inline connection
		auth, err := ctx.GetAuthValues(conn.Authentication)
		if err != nil {
			return nil, nil, err
		}
		_conn.Username = auth.Username.ValueStatic
		_conn.Password = auth.Password.ValueStatic
	}

	data := map[string]interface{}{
		"name":      ctx.Canary.Name,
		"namespace": ctx.Namespace,
		"username":  _conn.Username,
		"password":  _conn.Password,
		"domain":    getDomain(_conn.Username),
	}

	return _conn, data, nil
}

func (ctx *Context) GetConnection(conn v1.Connection) (*models.Connection, error) {
	connection, data, err := ctx.GetConnectionTemplate(conn)
	if err != nil {
		return nil, err
	}
	templater := ctx.NewStructTemplater(data, "template", nil)
	if err := templater.Walk(connection); err != nil {
		return nil, err
	}

	return connection, nil
}

func (ctx Context) TemplateStruct(o interface{}) error {
	templater := ctx.NewStructTemplater(ctx.Environment, "", ctx.GetContextualFunctions())
	return templater.Walk(o)
}

func (ctx Context) GetAuthValues(auth types.Authentication) (types.Authentication, error) {
	// in case nil we are sending empty string values for username and password
	if auth.IsEmpty() {
		return auth, nil
	}
	var err error

	if auth.Username.ValueStatic, err = ctx.GetEnvValueFromCache(auth.Username, ctx.GetNamespace()); err != nil {
		return auth, err
	}
	if auth.Password.ValueStatic, err = ctx.GetEnvValueFromCache(auth.Password, ctx.GetNamespace()); err != nil {
		return auth, err
	}
	return auth, nil
}

func (ctx *Context) HydrateConnectionByURL(connectionName string) (*models.Connection, error) {
	if connectionName == "" {
		return nil, nil
	}

	if !strings.HasPrefix(connectionName, "connection://") {
		return nil, nil
	}

	if ctx.DB() == nil {
		return nil, errors.New("db has not been initialized")
	}

	connection, err := ctx.Context.HydrateConnectionByURL(connectionName)
	if err != nil {
		return nil, err
	}

	// Connection name was explicitly provided but was not found.
	// That's an error.
	if connection == nil {
		return nil, fmt.Errorf("connection %s not found", connectionName)
	}

	return connection, nil
}

func New(ctx dutyCtx.Context, canary v1.Canary) *Context {
	if canary.Namespace == "" {
		canary.Namespace = "default"
	}

	ctx = ctx.WithObject(canary.ObjectMeta).
		WithName(fmt.Sprintf("Canary[%s/%s]", canary.Namespace, canary.Name)).
		WithDB(ctx.DB(), ctx.Pool())
	c := &Context{
		Context:     ctx,
		Namespace:   canary.Namespace,
		Canary:      canary,
		Environment: make(map[string]interface{}),
	}

	if c.Logger.IsLevelEnabled(4) || c.IsTrace() {
		c.Logger.SetMinLogLevel(2)
	} else if c.Logger.IsLevelEnabled(3) || c.IsDebug() {
		c.Logger.SetMinLogLevel(1)
	}
	return c
}

func (ctx *Context) IsDebug() bool {
	return ctx.Logger.IsLevelEnabled(3) || ctx.Canary.IsDebug() || ctx.IsTrace()
}

func (ctx *Context) IsTrace() bool {
	return ctx.Logger.IsLevelEnabled(4) || ctx.Canary.IsTrace()
}

func (ctx *Context) Debugf(format string, args ...interface{}) {
	if ctx.IsDebug() {
		ctx.Logger.Debugf(format, args...)
	}
}

func (ctx *Context) Tracef(format string, args ...interface{}) {
	if ctx.IsTrace() {
		ctx.Logger.Tracef(format, args...)
	}
}

func (ctx *Context) WithCheckResult(result *pkg.CheckResult) *Context {
	ctx = ctx.WithCheck(result.Check)
	ctx.Environment["duration"] = result.Duration
	for k, v := range result.Data {
		ctx.Environment[k] = v
	}
	return ctx
}

func (ctx *Context) WithDutyContext(c dutyCtx.Context) *Context {
	ctx.Context = c
	return ctx
}

func (ctx *Context) WithKubeconfig(input types.EnvVar) (Context, error) {
	c, err := ctx.Context.WithKubeconfig(input)
	ctx.Context = *c
	return *ctx, err
}

func (ctx *Context) WithCheck(check external.Check) *Context {
	env := make(map[string]any)

	checkID := ctx.Canary.GetCheckID(check.GetName())

	env["canary"] = map[string]any{
		"name":      ctx.Canary.GetName(),
		"namespace": ctx.Canary.GetNamespace(),
		"labels":    ctx.Canary.GetLabels(),
		"id":        ctx.Canary.GetPersistedID(),
	}

	env["check"] = map[string]any{
		"name":        check.GetName(),
		"id":          checkID,
		"description": check.GetDescription(),
		"labels":      check.GetLabels(),
		"endpoint":    check.GetEndpoint(),
	}
	return ctx.New(check.GetName(), env)
}

func (ctx *Context) WithEnvValues(environment map[string]interface{}) *Context {
	for k, v := range environment {
		ctx.Environment[k] = v
	}
	return ctx
}

func (ctx *Context) New(name string, environment map[string]interface{}) *Context {
	return &Context{
		Context:     ctx.Context.WithName(name),
		Namespace:   ctx.Namespace,
		Canary:      ctx.Canary,
		Environment: environment,
	}
}
