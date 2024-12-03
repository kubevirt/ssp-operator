package handler_hook

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

func (h *hook) Create(ctx context.Context, event event.TypedCreateEvent[client.Object], queue workqueue.TypedRateLimitingInterface[ctrl.Request]) {
	h.inner.Create(ctx, event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

func (h *hook) Update(ctx context.Context, event event.TypedUpdateEvent[client.Object], queue workqueue.TypedRateLimitingInterface[ctrl.Request]) {
	h.inner.Update(ctx, event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.ObjectNew)
	}})
}

func (h *hook) Delete(ctx context.Context, event event.TypedDeleteEvent[client.Object], queue workqueue.TypedRateLimitingInterface[ctrl.Request]) {
	h.inner.Delete(ctx, event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

func (h *hook) Generic(ctx context.Context, event event.TypedGenericEvent[client.Object], queue workqueue.TypedRateLimitingInterface[ctrl.Request]) {
	h.inner.Generic(ctx, event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

type queueHook struct {
	workqueue.TypedRateLimitingInterface[ctrl.Request]
	closure func(request ctrl.Request)
}

func (q *queueHook) Forget(item reconcile.Request) {
	q.closure(item)
	q.TypedRateLimitingInterface.Forget(item)
}

func (q *queueHook) NumRequeues(item reconcile.Request) int {
	q.closure(item)
	return q.TypedRateLimitingInterface.NumRequeues(item)
}

func (q *queueHook) AddRateLimited(item reconcile.Request) {
	q.closure(item)
	q.TypedRateLimitingInterface.Add(item)
}

func (q *queueHook) Add(item reconcile.Request) {
	q.closure(item)
	q.TypedRateLimitingInterface.Add(item)
}
