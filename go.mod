module github.com/dell/csi-baremetal-operator

go 1.16

require (
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/dell/csi-baremetal v0.2.2-beta
	github.com/go-logr/logr v0.3.0
	github.com/masterminds/semver v1.5.0
	github.com/openshift/api v0.0.0-20200618202633-7192180f496a
	github.com/stretchr/testify v1.5.1
	k8s.io/api v1.16.4
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v1.16.4
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.7.2
)

replace (
	k8s.io/api => k8s.io/api v0.19.2
	k8s.io/api v1.16.4 => k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.2
	k8s.io/apiserver => k8s.io/apiserver v0.19.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.2
	k8s.io/client-go => k8s.io/client-go v0.19.2
	k8s.io/client-go v1.16.4 => k8s.io/client-go v0.19.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.2
	k8s.io/code-generator => k8s.io/code-generator v0.19.2
	k8s.io/component-base => k8s.io/component-base v0.19.2
	k8s.io/cri-api => k8s.io/cri-api v0.19.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.2
	k8s.io/kubectl => k8s.io/kubectl v0.19.2
	k8s.io/kubelet => k8s.io/kubelet v0.19.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.2
	k8s.io/metrics => k8s.io/metrics v0.19.2
	k8s.io/node-api => k8s.io/node-api v0.19.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.2
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.19.2
	k8s.io/sample-controller => k8s.io/sample-controller v0.19.2
)
