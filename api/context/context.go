package context

import (
	gocontext "context"
	"errors"
	"fmt"
	"strings"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/flanksource/kommons"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
)

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
	db *gorm.DB
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

func (ctx *Context) GetEnvValueFromCache(env types.EnvVar) (string, error) {
	return duty.GetEnvValueFromCache(ctx.Kubernetes, env, ctx.Namespace)
}

func (ctx *Context) HydrateConnectionByURL(connectionName string) (*models.Connection, error) {
	if !strings.HasPrefix(connectionName, "connection://") {
		return nil, nil
	}
	if connectionName == "" {
		return nil, nil
	}

	if ctx.db == nil {
		return nil, errors.New("db has not been initialized")
	}

	connection, err := duty.HydratedConnectionByURL(ctx, ctx.db, ctx.Kubernetes, ctx.Namespace, connectionName)
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

func New(client *kommons.Client, kubernetes kubernetes.Interface, db *gorm.DB, canary v1.Canary) *Context {
	if canary.Namespace == "" {
		canary.Namespace = "default"
	}

	return &Context{
		db:          db,
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
	return ctx.Canary.IsDebug()
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

func (ctx *Context) New(environment map[string]interface{}) *Context {
	return &Context{
		Context:     ctx.Context,
		Kommons:     ctx.Kommons,
		Namespace:   ctx.Namespace,
		Canary:      ctx.Canary,
		Environment: environment,
		Logger:      ctx.Logger,
	}
}