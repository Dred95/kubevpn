package mesh

import (
	"fmt"
	"github.com/wencaiwulue/kubevpn/config"
	"github.com/wencaiwulue/kubevpn/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
)

func RemoveContainers(spec *v1.PodTemplateSpec) {
	for i := 0; i < len(spec.Spec.Volumes); i++ {
		if spec.Spec.Volumes[i].Name == config.SidecarEnvoyConfig {
			spec.Spec.Volumes = append(spec.Spec.Volumes[:i], spec.Spec.Volumes[i+1:]...)
		}
	}
	for i := 0; i < len(spec.Spec.Containers); i++ {
		for j := 0; j < len(spec.Spec.Containers[i].VolumeMounts); j++ {
			if spec.Spec.Containers[i].VolumeMounts[j].Name == config.SidecarEnvoyConfig {
				spec.Spec.Containers[i].VolumeMounts = append(spec.Spec.Containers[i].VolumeMounts[:j],
					spec.Spec.Containers[i].VolumeMounts[j+1:]...)
			}
		}
		if sets.NewString(config.SidecarEnvoyProxy, config.SidecarControlPlane, config.SidecarVPN).Has(spec.Spec.Containers[i].Name) {
			spec.Spec.Containers = append(spec.Spec.Containers[:i], spec.Spec.Containers[i+1:]...)
			i--
		}
	}
}

func AddMeshContainer(spec *v1.PodTemplateSpec, nodeId string, c util.PodRouteConfig) {
	// remove volume envoyConfig if already exist
	for i := 0; i < len(spec.Spec.Volumes); i++ {
		if spec.Spec.Volumes[i].Name == config.SidecarEnvoyConfig {
			spec.Spec.Volumes = append(spec.Spec.Volumes[:i], spec.Spec.Volumes[i+1:]...)
		}
	}
	// remove envoy proxy containers if already exist
	for i := 0; i < len(spec.Spec.Containers); i++ {
		if sets.NewString(config.SidecarEnvoyProxy, config.SidecarControlPlane, config.SidecarVPN).Has(spec.Spec.Containers[i].Name) {
			spec.Spec.Containers = append(spec.Spec.Containers[:i], spec.Spec.Containers[i+1:]...)
			i--
		}
	}
	zero := int64(0)
	t := true
	spec.Spec.Containers = append(spec.Spec.Containers, v1.Container{
		Name:    config.SidecarVPN,
		Image:   config.ImageServer,
		Command: []string{"/bin/sh", "-c"},
		Args: []string{
			"sysctl net.ipv4.ip_forward=1;" +
				"iptables -F;" +
				"iptables -P INPUT ACCEPT;" +
				"iptables -P FORWARD ACCEPT;" +
				//"iptables -t nat -A PREROUTING ! -p icmp ! -s 127.0.0.1 ! -d " + config.CIDR.String() + " -j DNAT --to 127.0.0.1:15006;" +
				//"iptables -t nat -A POSTROUTING ! -p icmp ! -s 127.0.0.1 ! -d " + config.CIDR.String() + " -j MASQUERADE;" +
				"iptables -t nat -A PREROUTING -i eth0 -p tcp --dport 80:60000 ! -s 127.0.0.1 ! -d " + config.CIDR.String() + " -j DNAT --to 127.0.0.1:15006;" +
				"iptables -t nat -A POSTROUTING -p tcp -m tcp --dport 80:60000 ! -s 127.0.0.1 ! -d " + config.CIDR.String() + " -j MASQUERADE;" +
				"iptables -t nat -A PREROUTING -i eth0 -p udp --dport 80:60000 ! -s 127.0.0.1 ! -d " + config.CIDR.String() + " -j DNAT --to 127.0.0.1:15006;" +
				"iptables -t nat -A POSTROUTING -p udp -m udp --dport 80:60000 ! -s 127.0.0.1 ! -d " + config.CIDR.String() + " -j MASQUERADE;" +
				"kubevpn serve -L 'tun://0.0.0.0:8421/" + c.TrafficManagerRealIP + ":8422?net=" + c.InboundPodTunIP + "&route=" + c.Route + "' --debug=true",
		},
		SecurityContext: &v1.SecurityContext{
			Capabilities: &v1.Capabilities{
				Add: []v1.Capability{
					"NET_ADMIN",
					//"SYS_MODULE",
				},
			},
			RunAsUser:  &zero,
			Privileged: &t,
		},
		Resources: v1.ResourceRequirements{
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse("128m"),
				v1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse("256m"),
				v1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		ImagePullPolicy: v1.PullAlways,
	})
	spec.Spec.Containers = append(spec.Spec.Containers, v1.Container{
		Name:    config.SidecarEnvoyProxy,
		Image:   config.ImageMesh,
		Command: []string{"envoy", "-l", "debug", "--config-yaml"},
		Args: []string{
			fmt.Sprintf(s, nodeId, nodeId /*c.TrafficManagerRealIP*/, "127.0.0.1"),
		},
		Resources: v1.ResourceRequirements{
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse("128m"),
				v1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse("256m"),
				v1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		ImagePullPolicy: v1.PullAlways,
	})
	spec.Spec.Containers = append(spec.Spec.Containers, v1.Container{
		Name:    config.SidecarControlPlane,
		Image:   config.ImageControlPlane,
		Command: []string{"envoy-xds-server"},
		Args:    []string{"--watchDirectoryFileName", "/etc/envoy/envoy-config.yaml"},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      config.VolumeEnvoyConfig,
				ReadOnly:  true,
				MountPath: "/etc/envoy",
			},
		},
		ImagePullPolicy: v1.PullAlways,
	})
	f := false
	spec.Spec.Volumes = append(spec.Spec.Volumes, v1.Volume{
		Name: config.VolumeEnvoyConfig,
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: config.PodTrafficManager,
				},
				Items: []v1.KeyToPath{
					{
						Key:  config.Envoy,
						Path: "envoy-config.yaml",
					},
				},
				Optional: &f,
			},
		},
	})
}
