module kubevirt.io/ssp-operator

go 1.13

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/prometheus-operator v0.41.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v0.0.0-20200917102736-0a191b5b9bb0 // release-4.5
	github.com/openshift/custom-resource-status v0.0.0-20200602122900-c002fd1547ca
	github.com/operator-framework/api v0.3.20
	github.com/operator-framework/operator-lib v0.2.0
	github.com/spf13/cobra v1.0.0
	k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v8.0.0+incompatible
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111
	kubevirt.io/controller-lifecycle-operator-sdk v0.1.1
	sigs.k8s.io/controller-runtime v0.6.3
)

replace k8s.io/client-go => k8s.io/client-go v0.18.6
