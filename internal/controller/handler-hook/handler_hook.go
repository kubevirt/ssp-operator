package handler_hook

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
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

func (h *hook) Create(event event.CreateEvent, queue workqueue.RateLimitingInterface) {
	h.inner.Create(event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

func (h *hook) Update(event event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	h.inner.Update(event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.ObjectNew)
	}})
}

func (h *hook) Delete(event event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	h.inner.Delete(event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

func (h *hook) Generic(event event.GenericEvent, queue workqueue.RateLimitingInterface) {
	h.inner.Generic(event, &queueHook{queue, func(request ctrl.Request) {
		h.hookFunc(request, event.Object)
	}})
}

var _ inject.Scheme = &hook{}

// The EnqueueRequestForOwner handler implements this interface.
func (h *hook) InjectScheme(scheme *runtime.Scheme) error {
	_, err := inject.SchemeInto(scheme, h.inner)
	return err
}

var _ inject.Mapper = &hook{}

// The EnqueueRequestForOwner handler implements this interface.
func (h *hook) InjectMapper(mapper meta.RESTMapper) error {
	_, err := inject.MapperInto(mapper, h.inner)
	return err
}

type queueHook struct {
	workqueue.RateLimitingInterface
	closure func(request ctrl.Request)
}

func (q *queueHook) Add(item interface{}) {
	q.closure(item.(ctrl.Request))
	q.RateLimitingInterface.Add(item)
}
