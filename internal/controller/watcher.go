package controller

import (
	"autouseal-vault/config"
	k8s2 "autouseal-vault/internal/k8s"
	"autouseal-vault/internal/vault"
	"context"
	"go.uber.org/dig"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"reflect"
	"sync"
)

type watchControllerParams struct {
	dig.In

	Cfg    *config.Config
	Ks     k8s2.KubeService
	Kr     k8s2.KubeRepo
	Unseal vault.Service
}

type watchController struct {
	p watchControllerParams
}

func (w *watchController) Watch(ctx context.Context) {
	watcher, err := w.p.Ks.WatchVaultServerPods(ctx)
	if err != nil {
		zap.S().Error(err)
		return
	}
	zap.S().Info("WatchController start")
	defer watcher.Stop()
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				// the channel got closed, so we need to restart
				zap.S().Warnf("WatchController hung up on us, need restart event watcher")
				return
			}
			if event.Type == watch.Added || event.Type == watch.Modified {
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					zap.S().Errorf("unexpected type %s, %+v", reflect.TypeOf(event.Object), event)
				} else {
					zap.S().Infof("%s(%s) in phase %s", pod.Name, pod.Status.PodIP, pod.Status.Phase)
					if pod.Status.Phase == "Running" {
						go w.p.Unseal.GetAndUnsealVault(ctx, pod.Status.PodIP)
					}
				}
			}
		case <-ctx.Done():
			zap.S().Infof("Exit from Watcher because the context is done")
			return
		}
	}
}

func (w *watchController) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ctx.Err() == nil {
				w.Watch(ctx)
			}
		}()
		wg.Wait()
	}
}
func NewWatchController(p watchControllerParams) Result {
	return Result{
		Controller: &watchController{
			p: p,
		},
	}
}
