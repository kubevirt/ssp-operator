module kubevirt.io/ssp-operator

go 1.16

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/fsnotify/fsnotify v1.4.9
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/openshift/api v0.0.0
	github.com/openshift/custom-resource-status v1.1.0
	github.com/operator-framework/api v0.9.2
	github.com/operator-framework/operator-lib v0.4.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.49.0
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	gomodules.xyz/jsonpatch/v2 v2.2.0
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20210709001253-0e1f9d693477
	kubevirt.io/client-go v0.43.0
	kubevirt.io/controller-lifecycle-operator-sdk v0.2.0
	kubevirt.io/qe-tools v0.1.7
	sigs.k8s.io/controller-runtime v0.9.3
)

replace (
	github.com/go-kit/kit => github.com/go-kit/kit v0.3.0
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2
	github.com/openshift/api => github.com/openshift/api v0.0.0-20210713130143-be21c6cb1bea // release-4.8
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // release-4.8
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a
	k8s.io/client-go => k8s.io/client-go v0.21.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.3
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.0
)
