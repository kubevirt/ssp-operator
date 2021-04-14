module kubevirt.io/ssp-operator

go 1.15

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/coreos/prometheus-operator v0.41.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.3.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f // release-4.6
	github.com/openshift/custom-resource-status v0.0.0-20200602122900-c002fd1547ca
	github.com/operator-framework/api v0.5.3
	github.com/operator-framework/operator-lib v0.4.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.1
	gomodules.xyz/jsonpatch/v2 v2.1.0
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	kubevirt.io/client-go v0.33.0
	kubevirt.io/controller-lifecycle-operator-sdk v0.1.3-0.20210112105647-bbf16167410b
	kubevirt.io/qe-tools v0.1.7
	sigs.k8s.io/controller-runtime v0.8.2
)

replace k8s.io/client-go => k8s.io/client-go v0.20.2
