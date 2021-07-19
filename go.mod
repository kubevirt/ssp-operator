module kubevirt.io/ssp-operator

go 1.15

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/coreos/prometheus-operator v0.41.1
	github.com/davecgh/go-spew v1.1.1
	github.com/fsnotify/fsnotify v1.4.9
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.3.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/openshift/api v0.0.0
	github.com/openshift/client-go v0.0.0
	github.com/openshift/custom-resource-status v0.0.0-20200602122900-c002fd1547ca
	github.com/operator-framework/api v0.5.3
	github.com/operator-framework/operator-lib v0.4.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	gomodules.xyz/jsonpatch/v2 v2.1.0
	k8s.io/api v0.20.6
	k8s.io/apiextensions-apiserver v0.20.6
	k8s.io/apimachinery v0.20.6
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	kubevirt.io/client-go v0.38.1
	kubevirt.io/containerized-data-importer v1.36.0
	kubevirt.io/controller-lifecycle-operator-sdk v0.2.0
	kubevirt.io/qe-tools v0.1.7
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	github.com/go-kit/kit => github.com/go-kit/kit v0.3.0
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2
	github.com/openshift/api => github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f // release-4.6
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200929181438-91d71ef2122c // release-4.6
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a
	k8s.io/client-go => k8s.io/client-go v0.20.6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.6
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.0
)
