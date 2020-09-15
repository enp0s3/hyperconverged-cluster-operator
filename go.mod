module github.com/kubevirt/hyperconverged-cluster-operator

go 1.13

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/evanphx/json-patch v4.9.0+incompatible // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.7
	github.com/imdario/mergo v0.3.9
	github.com/kubevirt/cluster-network-addons-operator v0.42.0
	github.com/kubevirt/controller-lifecycle-operator-sdk v0.0.5
	github.com/kubevirt/kubevirt-ssp-operator v1.2.1
	github.com/kubevirt/vm-import-operator v0.2.1
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/openshift/custom-resource-status v0.0.0-20200602122900-c002fd1547ca
	github.com/operator-framework/api v0.3.5
	github.com/operator-framework/operator-lifecycle-manager v0.0.0-20200321030439-57b580e57e88
	github.com/operator-framework/operator-sdk v0.18.0
	github.com/spf13/pflag v1.0.5
	golang.org/x/tools v0.0.0-20200616195046-dc31b401abb5
	k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	kubevirt.io/client-go v0.33.0-rc.1
	kubevirt.io/containerized-data-importer v1.22.0
	kubevirt.io/kubevirt v0.33.0-rc.1
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.4.0
)

// Pinned to kubernetes-1.16.4 to kubevirt.io/kubevirt v0.32.0-rc.2
replace (
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/apiserver => k8s.io/apiserver v0.16.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.16.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.4
	k8s.io/component-base => k8s.io/component-base v0.16.4
	k8s.io/cri-api => k8s.io/cri-api v0.16.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.16.4
	k8s.io/klog => k8s.io/klog v0.4.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.16.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.16.4
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.16.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.16.4
	k8s.io/kubectl => k8s.io/kubectl v0.16.4
	k8s.io/kubelet => k8s.io/kubelet v0.16.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.16.4
	k8s.io/metrics => k8s.io/metrics v0.16.4
	k8s.io/node-api => k8s.io/node-api v0.16.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.16.4
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.16.4
	k8s.io/sample-controller => k8s.io/sample-controller v0.16.4
)

replace (
	github.com/appscode/jsonpatch => github.com/appscode/jsonpatch v1.0.1
	github.com/coreos/prometheus-operator => github.com/coreos/prometheus-operator v0.35.0
	github.com/go-kit/kit => github.com/go-kit/kit v0.3.0
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.3.1
	github.com/openshift/machine-api-operator => github.com/openshift/machine-api-operator v0.2.1-0.20191025120018-fb3724fc7bdf
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.0.0-20190424153033-d3245f150225
	kubevirt.io/client-go => kubevirt.io/client-go v0.33.0-rc.1
)

// Aligning with https://github.com/kubevirt/containerized-data-importer/blob/release-v1.22.0
replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20191219222812-2987a591a72c
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20191125132246-f6563a70e19a
	github.com/openshift/library-go => github.com/mhenriks/library-go v0.0.0-20200116194830-9fcc1a687a9d
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2
)

// sigs.k8s.io/controller-runtime 0.6.0 requires k8s-* v0.18.2 but we are pinned
// to kubernetes-1.16.4  as for kubevirt.io/kubevirt v0.32.0-rc.2
// need github.com/operator-framework/api v0.3.5
replace (
	github.com/operator-framework/api => github.com/operator-framework/api v0.3.5
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.5.2
)

// cluster-network-addons-operator pulls in dependency on operator-sdk 0.39.2
// but since HCO is pinned to Kubernetes v0.16.4, it needs to stay on operator-sdk
// v0.17.0.
replace github.com/operator-framework/operator-sdk => github.com/operator-framework/operator-sdk v0.17.0

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm

// Pinned for compatibility with kubernetes-1.16.4
replace sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.2.4

replace vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787

replace bitbucket.org/ww/goautoneg => github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d
