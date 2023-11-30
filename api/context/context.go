package context

import (
	gocontext "context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	ctemplate "github.com/flanksource/commons/template"
	"github.com/flanksource/duty/connection"
	dutyCtx "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/flanksource/gomplate/v3"
	"github.com/flanksource/kommons"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
)

var DefaultContext dutyCtx.Context

type KubernetesContext struct {
	gocontext.Context
	Kommons     *kommons.Client
	Kubernetes  kubernetes.Interface
	Namespace   string
	Environment map[string]interface{}
	logger.Logger
}

type Context struct {
	gocontext.Context
	Kubernetes  kubernetes.Interface
	Kommons     *kommons.Client
	Namespace   string
	Canary      v1.Canary
	Environment map[string]interface{}
	logger.Logger
	db    *gorm.DB
	pool  *pgxpool.Pool
	cache map[string]any
}

func (ctx *Context) Duty() dutyCtx.Context {
	return dutyCtx.NewContext(gocontext.Background()).
		WithDB(ctx.db, ctx.pool).
		WithKubernetes(ctx.Kubernetes).
		WithNamespace(ctx.Namespace).
		WithObject(ctx.Canary.ObjectMeta)
}

func (ctx *Context) DB() *gorm.DB {
	if ctx.db == nil {
		return nil
	}

	return ctx.db.WithContext(ctx.Context)
}

func (ctx *Context) Pool() *pgxpool.Pool {
	return ctx.pool
}
func (ctx *Context) String() string {
	return fmt.Sprintf("%s/%s", ctx.Canary.Namespace, ctx.Canary.Name)
}

func (ctx *Context) WithTimeout(timeout time.Duration) (*Context, gocontext.CancelFunc) {
	return ctx.WithDeadline(time.Now().Add(timeout))
}

func (ctx *Context) WithDeadline(deadline time.Time) (*Context, gocontext.CancelFunc) {
	_ctx, fn := gocontext.WithDeadline(ctx.Context, deadline)
	ctx.Context = _ctx
	return ctx, fn
}

func (ctx *Context) GetEnvValueFromCache(env types.EnvVar, namespace ...string) (string, error) {
	return ctx.Duty().GetEnvValueFromCache(env, namespace...)
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
		tpl.Functions = make(map[string]func() any)
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

func (ctx *Context) GetConnection(conn v1.Connection) (*models.Connection, error) {
	var _conn *models.Connection
	var err error

	if _conn, err = ctx.HydrateConnectionByURL(conn.Connection); err != nil {
		return nil, err
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
			return nil, err
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
	templater := ctemplate.StructTemplater{
		Values: data,
		// access go values in template requires prefix everything with .
		// to support $(username) instead of $(.username) we add a function for each var
		ValueFunctions: true,
		DelimSets: []ctemplate.Delims{
			{Left: "{{", Right: "}}"},
			{Left: "$(", Right: ")"},
		},
		RequiredTag: "template",
	}
	if err := templater.Walk(_conn); err != nil {
		return nil, err
	}

	return _conn, nil
}

func (ctx Context) TemplateStruct(o interface{}) error {
	templater := gomplate.StructTemplater{
		Values: ctx.Environment,
		Funcs:  ctx.GetContextualFunctions(),
		// access go values in template requires prefix everything with .
		// to support $(username) instead of $(.username) we add a function for each var
		ValueFunctions: true,
		DelimSets: []gomplate.Delims{
			{Left: "{{", Right: "}}"},
			{Left: "$(", Right: ")"},
		},
	}
	return templater.Walk(o)
}

func (ctx Context) GetAuthValues(auth connection.Authentication) (connection.Authentication, error) {
	// in case nil we are sending empty string values for username and password
	if auth.IsEmpty() {
		return auth, nil
	}
	var err error

	if auth.Username.ValueStatic, err = ctx.GetEnvValueFromCache(auth.Username); err != nil {
		return auth, err
	}
	if auth.Password.ValueStatic, err = ctx.GetEnvValueFromCache(auth.Password); err != nil {
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

	if ctx.db == nil {
		return nil, errors.New("db has not been initialized")
	}

	connection, err := ctx.Duty().HydrateConnectionByURL(connectionName)
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

func NewKubernetesContext(client *kommons.Client, kubernetes kubernetes.Interface, namespace string) *KubernetesContext {
	if namespace == "" {
		namespace = "default"
	}
	return &KubernetesContext{
		Context:     gocontext.Background(),
		Kommons:     client,
		Kubernetes:  kubernetes,
		Namespace:   namespace,
		Environment: make(map[string]interface{}),
		Logger:      logger.StandardLogger(),
	}
}

func (ctx *KubernetesContext) Clone() *KubernetesContext {
	return &KubernetesContext{
		Context:     gocontext.Background(),
		Kommons:     ctx.Kommons,
		Kubernetes:  ctx.Kubernetes,
		Namespace:   ctx.Namespace,
		Environment: make(map[string]interface{}),
		Logger:      logger.StandardLogger(),
	}
}

func New(client *kommons.Client, kubernetes kubernetes.Interface, db *gorm.DB, pool *pgxpool.Pool, canary v1.Canary) *Context {
	if canary.Namespace == "" {
		canary.Namespace = "default"
	}

	return &Context{
		db:          db,
		pool:        pool,
		Context:     gocontext.Background(),
		Kommons:     client,
		Kubernetes:  kubernetes,
		Namespace:   canary.GetNamespace(),
		Canary:      canary,
		Environment: make(map[string]interface{}),
		Logger:      logger.StandardLogger(),
	}
}

func (ctx *Context) IsDebug() bool {
	return ctx.Canary.IsDebug() || ctx.IsTrace()
}

func (ctx *Context) IsTrace() bool {
	return ctx.Canary.IsTrace()
}

func (ctx *Context) Debugf(format string, args ...interface{}) {
	if ctx.IsDebug() {
		ctx.Logger.Infof(format, args...)
	}
}

func (ctx *Context) Tracef(format string, args ...interface{}) {
	if ctx.IsTrace() {
		ctx.Logger.Infof(format, args...)
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
	return ctx.New(env)
}

func (ctx *Context) WithEnvValues(environment map[string]interface{}) *Context {
	for k, v := range environment {
		ctx.Environment[k] = v
	}
	return ctx
}

func (ctx *Context) New(environment map[string]interface{}) *Context {
	return &Context{
		Context:     ctx.Context,
		Kommons:     ctx.Kommons,
		db:          ctx.db,
		pool:        ctx.pool,
		Kubernetes:  ctx.Kubernetes,
		Namespace:   ctx.Namespace,
		Canary:      ctx.Canary,
		Environment: environment,
		Logger:      ctx.Logger,
	}
}
