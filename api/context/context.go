package context

import (
	gocontext "context"
	"fmt"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
)

type Context struct {
	gocontext.Context
	Kommons     *kommons.Client
	Namespace   string
	Canary      v1.Canary
	Environment map[string]interface{}
	logger.Logger
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

func New(client *kommons.Client, canary v1.Canary) *Context {
	return &Context{
		Context:     gocontext.Background(),
		Kommons:     client,
		Namespace:   canary.GetNamespace(),
		Canary:      canary,
		Environment: make(map[string]interface{}),
		Logger:      logger.StandardLogger(),
	}
}

func (ctx *Context) IsDebug() bool {
	return ctx.Canary.Annotations != nil && ctx.Canary.Annotations["debug"] == "true"
}

func (ctx *Context) IsTrace() bool {
	return ctx.Canary.Annotations != nil && ctx.Canary.Annotations["trace"] == "true"
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
