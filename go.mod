module kubevirt.io/ssp-operator

go 1.13

require (
	github.com/coreos/prometheus-operator v0.41.1
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/operator-framework/operator-lib v0.1.0
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111
	sigs.k8s.io/controller-runtime v0.6.2
)
