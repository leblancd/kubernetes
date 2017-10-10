/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package staticpod

import (
	"net"
	"net/url"
	"reflect"
	"sort"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

func TestComponentResources(t *testing.T) {
	a := ComponentResources("250m")
	if a.Requests == nil {
		t.Errorf(
			"failed componentResources, return value was nil",
		)
	}
}

func TestComponentProbe(t *testing.T) {
	var tests = []struct {
		name      string
		cfg       *kubeadmapi.MasterConfiguration
		component string
		port      int
		path      string
		scheme    v1.URIScheme
	}{
		{
			name: "default apiserver advertise address with http",
			cfg: &kubeadmapi.MasterConfiguration{
				API: kubeadmapi.API{
					AdvertiseAddress: "",
				},
			},
			component: kubeadmconstants.KubeAPIServer,
			port:      1,
			path:      "foo",
			scheme:    v1.URISchemeHTTP,
		},
		{
			name: "default apiserver advertise address with https",
			cfg: &kubeadmapi.MasterConfiguration{
				API: kubeadmapi.API{
					AdvertiseAddress: "",
				},
			},
			component: kubeadmconstants.KubeAPIServer,
			port:      2,
			path:      "bar",
			scheme:    v1.URISchemeHTTPS,
		},
		{
			name: "valid ipv4 apiserver advertise address with http",
			cfg: &kubeadmapi.MasterConfiguration{
				API: kubeadmapi.API{
					AdvertiseAddress: "1.2.3.4",
				},
			},
			component: kubeadmconstants.KubeAPIServer,
			port:      1,
			path:      "foo",
			scheme:    v1.URISchemeHTTP,
		},
		{
			name: "valid IPv4 scheduler probe",
			cfg: &kubeadmapi.MasterConfiguration{
				SchedulerExtraArgs: map[string]string{"address": "1.2.3.4"},
			},
			component: kubeadmconstants.KubeScheduler,
			port:      1,
			path:      "foo",
			scheme:    v1.URISchemeHTTP,
		},
		{
			name: "valid etcd probe using listen-client-urls IPv4 addresses",
			cfg: &kubeadmapi.MasterConfiguration{
				Etcd: kubeadmapi.Etcd{
					ExtraArgs: map[string]string{
						"listen-client-urls": "http://1.2.3.4:2379,http://4.3.2.1:2379"},
				},
			},
			component: kubeadmconstants.Etcd,
			port:      1,
			path:      "foo",
			scheme:    v1.URISchemeHTTP,
		},
		{
			name: "valid IPv4 etcd probe using hostname for listen-client-urls",
			cfg: &kubeadmapi.MasterConfiguration{
				Etcd: kubeadmapi.Etcd{
					ExtraArgs: map[string]string{
						"listen-client-urls": "http://localhost:2379"},
				},
			},
			component: kubeadmconstants.Etcd,
			port:      1,
			path:      "foo",
			scheme:    v1.URISchemeHTTP,
		},
	}
	for _, rt := range tests {
		actual := ComponentProbe(rt.cfg, rt.component, rt.port, rt.path, rt.scheme)
		switch {
		case rt.component == kubeadmconstants.KubeAPIServer:
			if rt.cfg.API.AdvertiseAddress == "" &&
				actual.Handler.HTTPGet.Host != "127.0.0.1" {
				t.Errorf("%s test case failed:\n\texpected: %s\n\t  actual: %s",
					rt.name, "127.0.0.1",
					actual.Handler.HTTPGet.Host)
			}
			if rt.cfg.API.AdvertiseAddress != "" &&
				actual.Handler.HTTPGet.Host != rt.cfg.API.AdvertiseAddress {
				t.Errorf("%s test case failed:\n\texpected: %s\n\t  actual: %s",
					rt.name, rt.cfg.API.AdvertiseAddress,
					actual.Handler.HTTPGet.Host)
			}
		case rt.component == kubeadmconstants.KubeScheduler:
			if actual.Handler.HTTPGet.Host != rt.cfg.SchedulerExtraArgs["address"] {
				t.Errorf("%s test case failed:\n\texpected: %s\n\t  actual: %s",
					rt.name, rt.cfg.SchedulerExtraArgs["address"],
					actual.Handler.HTTPGet.Host)
			}
		case rt.component == kubeadmconstants.KubeControllerManager:
			if actual.Handler.HTTPGet.Host != rt.cfg.ControllerManagerExtraArgs["address"] {
				t.Errorf("%s test case failed:\n\texpected: %s\n\t  actual: %s",
					rt.name, rt.cfg.ControllerManagerExtraArgs["address"],
					actual.Handler.HTTPGet.Host)
			}
		case rt.component == kubeadmconstants.Etcd:
			arg, exists := rt.cfg.Etcd.ExtraArgs["listen-client-urls"]
			if exists {
				u, err := url.Parse(arg)
				if err != nil || u.Hostname() == "" {
					if actual.Handler.HTTPGet.Host != "127.0.0.1" {
						t.Errorf("%s test case failed:\n\texpected: %s\n\t  actual: %s",
							rt.name, "127.0.0.1", actual.Handler.HTTPGet.Host)
					}
				}
				if addr := net.ParseIP(u.Hostname()); addr != nil {
					if actual.Handler.HTTPGet.Host != addr.String() {
						t.Errorf("%s test case failed:\n\texpected: %s\n\t  actual: %s",
							rt.name, addr.String(), actual.Handler.HTTPGet.Host)
					}
				} else {
					var ip net.IP
					addrs, _ := net.LookupIP(u.Hostname())
					for _, addr := range addrs {
						if addr.To4() != nil {
							ip = addr
							break
						}
						if addr.To16() != nil && ip == nil {
							ip = addr
						}
					}
					if actual.Handler.HTTPGet.Host != ip.String() {
						t.Errorf("%s test case failed:\n\texpected: %s\n\t  actual: %s",
							rt.name, ip.String(), actual.Handler.HTTPGet.Host)
					}
				}
			}
		}
		if actual.Handler.HTTPGet.Port != intstr.FromInt(rt.port) {
			t.Errorf("%s test case failed:\n\texpected: %v\n\t  actual: %v",
				rt.name, rt.port,
				actual.Handler.HTTPGet.Port)
		}
		if actual.Handler.HTTPGet.Path != rt.path {
			t.Errorf("%s test case failed:\n\texpected: %s\n\t  actual: %s",
				rt.name, rt.path,
				actual.Handler.HTTPGet.Path)
		}
		if actual.Handler.HTTPGet.Scheme != rt.scheme {
			t.Errorf("%s test case failed:\n\texpected: %v\n\t  actual: %v",
				rt.name, rt.scheme,
				actual.Handler.HTTPGet.Scheme)
		}
	}
}

func TestComponentPod(t *testing.T) {
	var tests = []struct {
		name     string
		expected v1.Pod
	}{
		{
			name: "foo",
			expected: v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "foo",
					Namespace:   "kube-system",
					Annotations: map[string]string{"scheduler.alpha.kubernetes.io/critical-pod": ""},
					Labels:      map[string]string{"component": "foo", "tier": "control-plane"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "foo",
						},
					},
					HostNetwork: true,
					Volumes:     []v1.Volume{},
				},
			},
		},
	}

	for _, rt := range tests {
		c := v1.Container{Name: rt.name}
		actual := ComponentPod(c, []v1.Volume{})
		if !reflect.DeepEqual(rt.expected, actual) {
			t.Errorf(
				"failed componentPod:\n\texpected: %v\n\t  actual: %v",
				rt.expected,
				actual,
			)
		}
	}
}

func TestNewVolume(t *testing.T) {
	hostPathDirectoryOrCreate := v1.HostPathDirectoryOrCreate
	var tests = []struct {
		name     string
		path     string
		expected v1.Volume
		pathType *v1.HostPathType
	}{
		{
			name: "foo",
			path: "/etc/foo",
			expected: v1.Volume{
				Name: "foo",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/etc/foo",
						Type: &hostPathDirectoryOrCreate,
					},
				},
			},
			pathType: &hostPathDirectoryOrCreate,
		},
	}

	for _, rt := range tests {
		actual := NewVolume(rt.name, rt.path, rt.pathType)
		if !reflect.DeepEqual(actual, rt.expected) {
			t.Errorf(
				"failed newVolume:\n\texpected: %v\n\t  actual: %v",
				rt.expected,
				actual,
			)
		}
	}
}

func TestNewVolumeMount(t *testing.T) {
	var tests = []struct {
		name     string
		path     string
		ro       bool
		expected v1.VolumeMount
	}{
		{
			name: "foo",
			path: "/etc/foo",
			ro:   false,
			expected: v1.VolumeMount{
				Name:      "foo",
				MountPath: "/etc/foo",
				ReadOnly:  false,
			},
		},
		{
			name: "bar",
			path: "/etc/foo/bar",
			ro:   true,
			expected: v1.VolumeMount{
				Name:      "bar",
				MountPath: "/etc/foo/bar",
				ReadOnly:  true,
			},
		},
	}

	for _, rt := range tests {
		actual := NewVolumeMount(rt.name, rt.path, rt.ro)
		if !reflect.DeepEqual(actual, rt.expected) {
			t.Errorf(
				"failed newVolumeMount:\n\texpected: %v\n\t  actual: %v",
				rt.expected,
				actual,
			)
		}
	}
}

func TestGetExtraParameters(t *testing.T) {
	var tests = []struct {
		overrides map[string]string
		defaults  map[string]string
		expected  []string
	}{
		{
			overrides: map[string]string{
				"admission-control": "NamespaceLifecycle,LimitRanger",
			},
			defaults: map[string]string{
				"admission-control":     "NamespaceLifecycle",
				"insecure-bind-address": "127.0.0.1",
				"allow-privileged":      "true",
			},
			expected: []string{
				"--admission-control=NamespaceLifecycle,LimitRanger",
				"--insecure-bind-address=127.0.0.1",
				"--allow-privileged=true",
			},
		},
		{
			overrides: map[string]string{
				"admission-control": "NamespaceLifecycle,LimitRanger",
			},
			defaults: map[string]string{
				"insecure-bind-address": "127.0.0.1",
				"allow-privileged":      "true",
			},
			expected: []string{
				"--admission-control=NamespaceLifecycle,LimitRanger",
				"--insecure-bind-address=127.0.0.1",
				"--allow-privileged=true",
			},
		},
	}

	for _, rt := range tests {
		actual := GetExtraParameters(rt.overrides, rt.defaults)
		sort.Strings(actual)
		sort.Strings(rt.expected)
		if !reflect.DeepEqual(actual, rt.expected) {
			t.Errorf("failed getExtraParameters:\nexpected:\n%v\nsaw:\n%v", rt.expected, actual)
		}
	}
}
