package dns

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wencaiwulue/kubevpn/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetDNSServiceIPFromPod(clientset *kubernetes.Clientset, restclient *rest.RESTClient, config *rest.Config, podName, namespace string) string {
	if ip, err := getDNSIP(clientset); err == nil && len(ip) != 0 {
		return ip
	}
	if ip, err := util.Shell(clientset, restclient, config, util.TrafficManager, namespace, "cat /etc/resolv.conf | grep nameserver | awk '{print$2}'"); err == nil && len(ip) != 0 {
		return ip
	}
	logrus.Fatal("this should not happened")
	return ""
}

func getDNSIP(clientset *kubernetes.Clientset) (string, error) {
	serviceList, err := clientset.CoreV1().Services(v1.NamespaceSystem).List(context.Background(), v1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", "kube-dns").String(),
	})
	if err != nil {
		return "", err
	}
	if len(serviceList.Items) == 0 {
		return "", errors.New("Not found kube-dns")
	}
	return serviceList.Items[0].Spec.ClusterIP, nil
}
