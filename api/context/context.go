package context

import (
	gocontext "context"
	"errors"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	k8sv1 "k8s.io/api/core/v1"
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
func (ctx *Context) GetCanaries(namespace string, canaryRef []k8sv1.LocalObjectReference) ([]v1.Canary, []string, error) {
	var innerCanaries []v1.Canary

	innerFail := false
	var innerMessage []string

	for _, canary := range canaryRef {
		innerCanary := v1.Canary{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Canary",
				APIVersion: "canaries.flanksource.com/v1",
			},
		}
		err := ctx.Kommons.Get(namespace, canary.Name, &innerCanary)
		logger.Infof("Accessing Canary %v/%v", namespace, canary.Name)
		if err != nil {
			innerFail = true
			innerMessage = append(innerMessage, fmt.Sprintf("Could not retrieve canary ref %v in %v: %v", canary.Name, namespace, err))
			break
		}
		if innerCanary.Name == "" {
			innerFail = true
			innerMessage = append(innerMessage, fmt.Sprintf("Could not retrieve canary ref %v in %v", canary.Name, namespace))
			break
		}
		innerCanaries = append(innerCanaries, innerCanary)
	}
	if innerFail {
		return innerCanaries, innerMessage, errors.New("error retrieving chained canaries")
	}
	return innerCanaries, innerMessage, nil
}