package handler_hook

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type HookFunc func(request ctrl.Request, obj client.Object)

func New(wrapped handler.EventHandler, hookFunc HookFunc) handler.EventHandler {
	return &hook{
		inner:    wrapped,
		hookFunc: hookFunc,
	}
}

type hook struct {
	inner    handler.EventHandler
	hookFunc HookFunc
}

var _ handler.EventHandler = &hook{}

func (h *hook) Create(ctx context.Context, event event.CreateEvent, queue workqueue.RateLimitingInterface) {
	h.inner.Create(ctx, event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

func (h *hook) Update(ctx context.Context, event event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	h.inner.Update(ctx, event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.ObjectNew)
	}})
}

func (h *hook) Delete(ctx context.Context, event event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	h.inner.Delete(ctx, event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

func (h *hook) Generic(ctx context.Context, event event.GenericEvent, queue workqueue.RateLimitingInterface) {
	h.inner.Generic(ctx, event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

type queueHook struct {
	workqueue.RateLimitingInterface
	closure func(request ctrl.Request)
}

func (q *queueHook) Add(item interface{}) {
	q.closure(item.(ctrl.Request))
	q.RateLimitingInterface.Add(item)
}
