package main

import (
	"autouseal-vault/config"
	controller2 "autouseal-vault/internal/controller"
	"autouseal-vault/internal/http"
	k8s2 "autouseal-vault/internal/k8s"
	"autouseal-vault/internal/vault"
	"context"
	"go.uber.org/dig"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	buildTime = "now"
	version   = "local_developer"
)

func main() {
	config.GetCfg()
	ctx, cancelFunction := context.WithCancel(context.Background())
	container := dig.New()
	container.Provide(config.GetCfg)       //nolint:errcheck
	container.Provide(k8s2.NewKubeService) //nolint:errcheck
	container.Provide(k8s2.NewKubeRepo)    //nolint:errcheck
	container.Provide(http.NewWebServer)   //nolint:errcheck
	container.Provide(vault.New)           //nolint:errcheck
	container.Provide(controller2.NewWatchController)

	zap.S().Infof("autounseal startint. Version: %s. (BuiltTime: %s)\n", version, buildTime)

	if err := container.Invoke(func(webServer http.WebServer) {
		webServer.Start()
	}); err != nil {
		zap.S().Fatal(err)
	}

	defer func() {
		zap.S().Info("Main Defer: canceling context")
		cancelFunction()
		time.Sleep(time.Second * 5)
	}()

	if err := container.Invoke(func(cfg *config.Config, unseal vault.Service) {
		go func() {
			zap.S().Info("GetUnsealKey started")
			unseal.GetUnsealKey(ctx)
			ticker := time.NewTicker(time.Second * time.Duration(12*cfg.Interval))
			for {
				select {
				case <-ctx.Done():
					zap.S().Info("finish main context")
					return
				case t := <-ticker.C:
					zap.S().Info("GetUnsealKey start")
					unseal.GetUnsealKey(ctx)
					zap.S().Info("GetUnsealKey finish:", time.Since(t))
				}
			}
		}()
	}); err != nil {
		zap.S().Fatal(err)
	}

	if err := container.Invoke(func(cfg *config.Config, unseal vault.Service) {
		go func() {
			zap.S().Info("GetAndUnsealVault started")
			ticker := time.NewTicker(time.Second * time.Duration(cfg.Interval))
			for {
				select {
				case <-ctx.Done():
					zap.S().Info("finish main context")
					return
				case t := <-ticker.C:
					zap.S().Info("start GetAndUnsealVault")
					unseal.GetPod4unseal(ctx)
					zap.S().Info("finish GetAndUnsealVault:", time.Since(t))
				}
			}
		}()
	}); err != nil {
		zap.S().Fatal(err)
	}

	if err := container.Invoke(func(ctlList controller2.List) {
		for _, ctl := range ctlList.Controllers {
			go ctl.Start(ctx)
		}
	}); err != nil {
		zap.S().Fatal(err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	sigName := <-signals
	zap.S().Infof("Received SIGNAL - %s. Terminating...", sigName)
}
