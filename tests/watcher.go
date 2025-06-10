package tests

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"k8s.io/client-go/tools/cache"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
)

type SspWatch interface {
	Stop()
	Updates() <-chan *ssp.SSP
	Error() error
}

func StartWatch(listerWatcher cache.ListerWatcher) (SspWatch, error) {
	listObj, err := listerWatcher.List(v1.ListOptions{ResourceVersion: ""})
	if err != nil {
		return nil, err
	}

	list, err := meta.ListAccessor(listObj)
	if err != nil {
		return nil, err
	}

	watch := &sspWatch{
		listerWatcher: listerWatcher,
		stopCh:        make(chan struct{}),
		updateCh:      make(chan *ssp.SSP),
		lastVersion:   list.GetResourceVersion(),
	}
	go watch.startWatch()
	return watch, nil
}

type sspWatch struct {
	listerWatcher cache.ListerWatcher
	stopCh        chan struct{}
	updateCh      chan *ssp.SSP
	atomicError   atomic.Value
	lastVersion   string
}

func (s *sspWatch) Stop() {
	close(s.stopCh)
}

func (s *sspWatch) Updates() <-chan *ssp.SSP {
	return s.updateCh
}

func (s *sspWatch) Error() error {
	return s.atomicError.Load().(error)
}

const watchTimeout = 1 * time.Minute

func (s *sspWatch) startWatch() {
	defer func() {
		close(s.updateCh)
	}()

	for {
		timeoutSec := int64(watchTimeout.Seconds())
		options := v1.ListOptions{
			ResourceVersion: s.lastVersion,
			TimeoutSeconds:  &timeoutSec,
		}

		w, err := s.listerWatcher.Watch(options)
		if err != nil {
			s.atomicError.Store(err)
			return
		}

		err = s.handleWatch(w)
		if err != nil {
			if errors.Is(err, errStopWatch) {
				s.atomicError.Store(err)
			}
			return
		}
	}
}

var errStopWatch = errors.New("watch stopped")

func (s *sspWatch) handleWatch(w watch.Interface) error {
	defer w.Stop()
	for {
		select {
		case <-s.stopCh:
			return errStopWatch
		case event, ok := <-w.ResultChan():
			if !ok {
				return nil
			}
			if event.Type == watch.Error {
				err := apierrors.FromObject(event.Object)
				return err
			}
			sspObj, ok := event.Object.(*ssp.SSP)
			if !ok {
				panic("Watch should receive SSP type.")
			}

			if event.Type == watch.Added || event.Type == watch.Modified {
				select {
				case <-s.stopCh:
					return errStopWatch
				case s.updateCh <- sspObj:
					break
				}
			}
			s.lastVersion = sspObj.GetResourceVersion()
		}
	}
}

var ErrTimeout = fmt.Errorf("timed out")

func WatchChangesUntil(watch SspWatch, predicate func(updatedSsp *ssp.SSP) bool, timeout time.Duration) error {
	timeoutCh := time.After(timeout)
	for {
		select {
		case obj, ok := <-watch.Updates():
			if !ok {
				return watch.Error()
			}
			if predicate(obj) {
				return nil
			}
		case <-timeoutCh:
			return ErrTimeout
		}
	}
}
