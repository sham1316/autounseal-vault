package k8s

import (
	"autouseal-vault/config"
	"context"
	"flag"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"sync"
)

type KubeService interface {
	GetServiceList(ctx context.Context) (*v1.ServiceList, error)
	GetVaultActiveService(ctx context.Context) (*v1.Service, error)
	GetVaultHeadlessService(ctx context.Context) (*v1.Service, error)
	GetVaultServerPods(ctx context.Context) (*v1.PodList, error)
	WatchVaultServerPods(ctx context.Context) (watch.Interface, error)
	GetToken() string
	GetCA() []byte
}

type kubeService struct {
	Cfg             *config.Config
	k8sConfig       *rest.Config
	clientSet       *kubernetes.Clientset
	resourceVersion string
	labelSelector   labels.Selector
	sync.Mutex
}

func NewKubeService(cfg *config.Config) KubeService {
	k8sConfig := getConfig(cfg.InCluster, cfg.Kubeconfig)
	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		zap.S().Fatal(err)
	}
	labelSelector, _ := labels.Parse(cfg.K8S.VaultServerPodLabels)
	return &kubeService{
		Cfg:             cfg,
		k8sConfig:       k8sConfig,
		clientSet:       clientSet,
		labelSelector:   labelSelector,
		resourceVersion: "0",
	}
}

func getConfig(inCluster bool, kubeconfig string) *rest.Config {
	var config *rest.Config
	var err error
	if inCluster {
		config, err = rest.InClusterConfig()
	} else {
		if kubeconfig == "" {
			if home := homedir.HomeDir(); home != "" {
				kubeconfig = *flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
			} else {
				kubeconfig = *flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
			}
			flag.Parse()
		}
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		zap.S().Fatal(err)
	}
	return config
}
func (k *kubeService) GetToken() string {
	return k.k8sConfig.BearerToken
}

func (k *kubeService) GetCA() []byte {
	return k.k8sConfig.TLSClientConfig.CAData
}

func (k *kubeService) GetServiceList(ctx context.Context) (*v1.ServiceList, error) {
	k.Lock()
	defer k.Unlock()
	opt := metav1.ListOptions{}
	n, err := k.clientSet.CoreV1().Services(k.Cfg.K8S.Namespace).List(ctx, opt)
	k.resourceVersion = n.GetResourceVersion()
	return n, err
}

func (k *kubeService) GetVaultActiveService(ctx context.Context) (*v1.Service, error) {
	return k.getVaultService(ctx, k.Cfg.K8S.VaultActiveService)
}

func (k *kubeService) GetVaultHeadlessService(ctx context.Context) (*v1.Service, error) {
	return k.getVaultService(ctx, k.Cfg.K8S.VaultHeadlessService)
}

func (k *kubeService) getVaultService(ctx context.Context, name string) (*v1.Service, error) {
	k.Lock()
	defer k.Unlock()
	opt := metav1.GetOptions{}
	n, err := k.clientSet.CoreV1().Services(k.Cfg.K8S.Namespace).Get(ctx, name, opt)
	k.resourceVersion = n.GetResourceVersion()
	return n, err
}

func (k *kubeService) GetVaultServerPods(ctx context.Context) (*v1.PodList, error) {
	k.Lock()
	defer k.Unlock()
	opt := metav1.ListOptions{LabelSelector: k.labelSelector.String()}
	return k.clientSet.CoreV1().Pods(k.Cfg.K8S.Namespace).List(ctx, opt)
}

func (k *kubeService) WatchVaultServerPods(ctx context.Context) (watch.Interface, error) {
	k.Lock()
	defer k.Unlock()
	opt := metav1.ListOptions{LabelSelector: k.labelSelector.String(), ResourceVersion: k.resourceVersion}
	w, err := k.clientSet.CoreV1().Pods(k.Cfg.K8S.Namespace).Watch(ctx, opt)
	return w, err
}
