package k8s

import (
	"autouseal-vault/config"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
)

type KubeRepo interface {
	GetServiceList(ctx context.Context) *v1.ServiceList
	GetVaultActiveService(ctx context.Context) *v1.Service
	GetVaultHeadlessService(ctx context.Context) *v1.Service
	GetVaultServerPods(ctx context.Context) *v1.PodList
}

type kubeRepo struct {
	ks KubeService
}

func NewKubeRepo(ks KubeService, cfg *config.Config) KubeRepo {
	return &kubeRepo{
		ks: ks,
	}
}

func (kr *kubeRepo) GetServiceList(ctx context.Context) *v1.ServiceList {
	serviceList, err := kr.ks.GetServiceList(ctx)
	if err != nil {
		zap.S().Errorf("error GetServiceList: %v", err)
		return nil
	}
	return serviceList
}

func (kr *kubeRepo) GetVaultActiveService(ctx context.Context) *v1.Service {
	service, err := kr.ks.GetVaultActiveService(ctx)
	if err != nil {
		zap.S().Errorf("error GetVaultService: %v", err)
		return nil
	}
	return service
}

func (kr *kubeRepo) GetVaultHeadlessService(ctx context.Context) *v1.Service {
	service, err := kr.ks.GetVaultHeadlessService(ctx)
	if err != nil {
		zap.S().Errorf("error GetVaultService: %v", err)
		return nil
	}
	return service
}
func (kr *kubeRepo) GetVaultServerPods(ctx context.Context) *v1.PodList {
	pods, err := kr.ks.GetVaultServerPods(ctx)
	if err != nil {
		zap.S().Errorf("error GetVaultService: %v", err)
		return nil
	}
	return pods
}
