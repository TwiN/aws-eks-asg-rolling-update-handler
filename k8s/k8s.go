package k8s

import (
	"os"
	"path/filepath"

	"github.com/TwiN/aws-eks-asg-rolling-update-handler/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// CreateClientSet Creates a Kubernetes ClientSet for authenticating with a cluster
// If the current environment is dev, use the user's kubeconfig
// If it isn't, then it means that the application is inside the cluster, which means
// we'll use the service account token
func CreateClientSet() (*kubernetes.Clientset, error) {
	var cfg *rest.Config
	if config.Get().Environment == "dev" {
		var kubeConfig string
		if home := homeDir(); home != "" {
			kubeConfig = filepath.Join(home, ".kube", "config")
		} else {
			panic("Home directory not found")
		}
		// use the current context in kubeconfig
		clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			return nil, err
		}
		cfg = clientConfig
	} else {
		clientConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		cfg = clientConfig
	}
	return kubernetes.NewForConfig(cfg)
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return os.Getenv("USERPROFILE") // windows
}
