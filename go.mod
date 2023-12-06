module kubevirt.io/ssp-operator

go 1.20

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/go-logr/logr v1.3.0
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/kubevirt/monitoring/pkg/metrics/parser v0.0.0-20230706095033-373a95665d5a
	github.com/machadovilaca/operator-observability v0.0.5
	github.com/onsi/ginkgo/v2 v2.13.1
	github.com/onsi/gomega v1.30.0
	github.com/openshift/api v0.0.0-20230503133300-8bbcb7ca7183 // release-4.13
	github.com/openshift/custom-resource-status v1.1.2
	github.com/openshift/library-go v0.0.0-20230228181805-0899dfdba7d2 // release-4.13
	github.com/operator-framework/api v0.17.3
	github.com/operator-framework/operator-lib v0.11.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.64.1
	github.com/prometheus/client_golang v1.14.0
	github.com/prometheus/client_model v0.4.0
	github.com/prometheus/common v0.41.0
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/tektoncd/pipeline v0.41.2
	gomodules.xyz/jsonpatch/v2 v2.2.0
	k8s.io/api v0.27.7
	k8s.io/apiextensions-apiserver v0.26.10
	k8s.io/apimachinery v0.27.7
	k8s.io/client-go v0.26.10
	k8s.io/kube-aggregator v0.26.10
	k8s.io/utils v0.0.0-20230505201702-9f6742963106
	kubevirt.io/api v1.0.1
	kubevirt.io/containerized-data-importer-api v1.57.0
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.2.4
	kubevirt.io/qe-tools v0.1.8
	kubevirt.io/ssp-operator/api v0.0.0
	sigs.k8s.io/controller-runtime v0.14.7
	sigs.k8s.io/kustomize/api v0.13.1
	sigs.k8s.io/kustomize/kyaml v0.14.0
	sigs.k8s.io/yaml v1.3.0
)

require (
	contrib.go.opencensus.io/exporter/ocagent v0.7.1-0.20200907061046-05415f1de66d // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blendle/zapdriver v1.3.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-containerregistry v0.12.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.11.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/oauth2 v0.13.0 // indirect
	golang.org/x/sync v0.4.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.14.0 // indirect
	google.golang.org/api v0.100.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/grpc v1.59.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiserver v0.26.10 // indirect
	k8s.io/component-base v0.26.10 // indirect
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230308215209-15aac26d736a // indirect
	knative.dev/pkg v0.0.0-20230221152827-2d84369c105d // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.26.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.10

	kubevirt.io/ssp-operator/api => ./api

	// Redirect nonexisting URL "vbom.ml/util".
	// TODO: Remove when github.com/openshift/library-go will be updated to 4.15
	vbom.ml/util => github.com/fvbommel/util v0.0.3
)
