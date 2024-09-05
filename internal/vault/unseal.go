package vault

import (
	"autouseal-vault/config"
	k8s2 "autouseal-vault/internal/k8s"
	"context"
	"fmt"
	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
	"go.uber.org/zap"
	"sync"
	"time"
)

type UnsealKeys map[string]string

type Service interface {
	GetUnsealKey(ctx context.Context)
	GetAndUnsealVault(ctx context.Context, ip string)
	GetPod4unseal(ctx context.Context)
}

type vaultService struct {
	cfg        *config.Config
	kr         k8s2.KubeRepo
	token      string
	ca         []byte
	unsealKeys UnsealKeys
	sync.Mutex
}

func New(cfg *config.Config, ks k8s2.KubeService, kr k8s2.KubeRepo) Service {
	return &vaultService{
		cfg:        cfg,
		token:      ks.GetToken(),
		ca:         ks.GetCA(),
		kr:         kr,
		unsealKeys: make(UnsealKeys),
	}
}
func (vs *vaultService) GetUnsealKey(ctx context.Context) {
	config := vault.DefaultConfig()
	config.Address = fmt.Sprintf("%s://%s:%d", vs.cfg.K8S.VaultSchema, vs.cfg.K8S.VaultActiveService, vs.cfg.K8S.VaultPort)
	config.Timeout = 60 * time.Second
	client, err := vault.NewClient(config)

	if err != nil {
		zap.S().Error(err)
	}
	k8sAuth, err := auth.NewKubernetesAuth(
		vs.cfg.K8S.VaultRole,
		auth.WithServiceAccountToken(vs.token))
	if vs.cfg.InCluster {
		k8sAuth, err = auth.NewKubernetesAuth(
			vs.cfg.K8S.VaultRole,
			auth.WithServiceAccountTokenPath("/var/run/secrets/kubernetes.io/serviceaccount/token"))
	}

	if err != nil {
		zap.S().Errorf("unable to initialize Kubernetes auth method: %v", err)
		return
	}

	authInfo, err := client.Auth().Login(ctx, k8sAuth)
	if err != nil {
		zap.S().Errorf("una111ble to log in with Kubernetes auth: %v", err)
		return
	}
	if authInfo == nil {
		zap.S().Errorf("no auth info was returned after login")
		return
	}

	secret, err := client.KVv2("internal").Get(context.Background(), "unseal")
	if err != nil {
		zap.S().Errorf("unable to read secret: %v", err)
		return
	}
	vs.Lock()
	defer vs.Unlock()
	for key, value := range secret.Data {
		vs.unsealKeys[key] = value.(string)
		zap.S().Infof("unseal: %s(%s...)", key, (value.(string))[0:1])
	}
}

func (vs *vaultService) GetPod4unseal(ctx context.Context) {
	pods := vs.kr.GetVaultServerPods(ctx)
	for _, pod := range pods.Items {
		zap.S().Info("vault-server pod ip:", pod.Status.PodIP)
		vs.GetAndUnsealVault(ctx, pod.Status.PodIP)
	}
}

func (vs *vaultService) GetAndUnsealVault(ctx context.Context, ip string) {
	vs.Lock()
	defer vs.Unlock()
	config := vault.DefaultConfig()
	config.Address = fmt.Sprintf("%s://%s:%d", vs.cfg.K8S.VaultSchema, ip, vs.cfg.K8S.VaultPort)
	config.Timeout = 20 * time.Second
	tls := &vault.TLSConfig{
		Insecure: true,
	}
	config.ConfigureTLS(tls)
	client, err := vault.NewClient(config)
	if err != nil {
		zap.S().Error(err)
		return
	}
	status, err := client.Sys().SealStatusWithContext(ctx)
	if err != nil {
		zap.S().Error("can't get Health status:", err)
		return
	}
	zap.S().Infof("%s: status: init=%t, sealed=%t, total=%d, threshold=%d", ip, status.Initialized, status.Sealed, status.N, status.T)
	if status.Initialized == true && status.Sealed == true {
		zap.S().Warnf("%s: need unsealed", ip)
		for _, unsealKey := range vs.unsealKeys {
			client.Sys().UnsealWithContext(ctx, unsealKey)
			status, err := client.Sys().SealStatusWithContext(ctx)
			if err != nil {
				zap.S().Error("can't get seal status:", err)
				return
			}
			client.Sys().UnsealWithContext(ctx, unsealKey)
			zap.S().Infof("%s: status: init=%t, sealed=%t, total=%d, threshold=%d", ip, status.Initialized, status.Sealed, status.N, status.T)
			if status.Sealed == false {
				break
			}
		}
	}
}
