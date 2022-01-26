module kubevirt.io/ssp-operator

go 1.16

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/fsnotify/fsnotify v1.5.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/api v0.0.0
	github.com/openshift/custom-resource-status v1.1.0
	github.com/operator-framework/api v0.11.1
	github.com/operator-framework/operator-lib v0.9.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.51.2
	github.com/prometheus/client_golang v1.12.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	gomodules.xyz/jsonpatch/v2 v2.2.0
	k8s.io/api v0.22.6
	k8s.io/apiextensions-apiserver v0.22.6
	k8s.io/apimachinery v0.22.6
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20211208161948-7d6a63dca704
	kubevirt.io/api v0.49.0
	kubevirt.io/client-go v0.49.0
	kubevirt.io/containerized-data-importer-api v1.42.1
	kubevirt.io/controller-lifecycle-operator-sdk v0.2.3
	kubevirt.io/qe-tools v0.1.7
	kubevirt.io/ssp-operator/api v0.0.0
	sigs.k8s.io/controller-runtime v0.10.3
)

replace (
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2
	github.com/openshift/api => github.com/openshift/api v0.0.0-20211028023115-7224b732cc14 // release-4.9
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20210831095141-e19a065e79f7 // release-4.9
	k8s.io/client-go => k8s.io/client-go v0.22.6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.6

	kubevirt.io/ssp-operator/api => ./api
)
