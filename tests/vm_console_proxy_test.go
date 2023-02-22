package tests

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	vm_console_proxy "kubevirt.io/ssp-operator/internal/operands/vm-console-proxy"
)

var _ = Describe("VM Console Proxy Operand", func() {
	var (
		clusterRoleResource        testResource
		clusterRoleBindingResource testResource
		serviceAccountResource     testResource
		serviceResource            testResource
		deploymentResource         testResource
		configMapResource          testResource
		routeResource              testResource
	)

	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()

		updateSsp(func(foundSsp *ssp.SSP) {
			if foundSsp.GetAnnotations() == nil {
				foundSsp.Annotations = make(map[string]string)
			}

			namespace := strategy.GetVmConsoleProxyNamespace()

			foundSsp.Annotations[vm_console_proxy.EnableAnnotation] = "true"
			foundSsp.Annotations[vm_console_proxy.VmConsoleProxyNamespaceAnnotation] = namespace
		})

		expectedLabels := expectedLabelsFor("vm-console-proxy", "vm-console-proxy")
		clusterRoleResource = testResource{
			Name:           "vm-console-proxy",
			Resource:       &rbac.ClusterRole{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(role *rbac.ClusterRole) {
				role.Rules[0].Verbs = []string{"watch"}
			},
			EqualsFunc: func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		clusterRoleBindingResource = testResource{
			Name:           "vm-console-proxy",
			Resource:       &rbac.ClusterRoleBinding{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(roleBinding *rbac.ClusterRoleBinding) {
				roleBinding.Subjects = nil
			},
			EqualsFunc: func(old *rbac.ClusterRoleBinding, new *rbac.ClusterRoleBinding) bool {
				return reflect.DeepEqual(old.RoleRef, new.RoleRef) &&
					reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}
		serviceAccountResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetVmConsoleProxyNamespace(),
			Resource:       &core.ServiceAccount{},
			ExpectedLabels: expectedLabels,
		}
		serviceResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetVmConsoleProxyNamespace(),
			Resource:       &core.Service{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(service *core.Service) {
				service.Spec.Ports[0].Port = 1443
				service.Spec.Ports[0].TargetPort = intstr.FromInt(18768)
			},
			EqualsFunc: func(old *core.Service, new *core.Service) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		deploymentResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetVmConsoleProxyNamespace(),
			Resource:       &apps.Deployment{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(deployment *apps.Deployment) {
				deployment.Spec.Replicas = pointer.Int32(0)
			},
			EqualsFunc: func(old *apps.Deployment, new *apps.Deployment) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		configMapResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetVmConsoleProxyNamespace(),
			Resource:       &core.ConfigMap{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(configMap *core.ConfigMap) {
				configMap.Data = map[string]string{"tls-profile-v1alpha1.json": "{}"}
			},
			EqualsFunc: func(old *core.ConfigMap, new *core.ConfigMap) bool {
				return reflect.DeepEqual(old.Immutable, new.Immutable) &&
					reflect.DeepEqual(old.Data, new.Data) &&
					reflect.DeepEqual(old.BinaryData, new.BinaryData)
			},
		}
		routeResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetVmConsoleProxyNamespace(),
			Resource:       &routev1.Route{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(route *routev1.Route) {
				route.Spec.TLS = nil
			},
			EqualsFunc: func(old, new *routev1.Route) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}

		waitUntilDeployed()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
	})

	Context("Resource creation", func() {
		DescribeTable("created cluster resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, res.GetKey(), resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue(), "owner annotations are missing")
		},
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] service account", &serviceAccountResource),
			Entry("[test_id:TODO] service", &serviceResource),
			Entry("[test_id:TODO] deployment", &deploymentResource),
			Entry("[test_id:TODO] config map", &configMapResource),
			Entry("[test_id:TODO] route", &routeResource),
		)

		DescribeTable("should set app labels", expectAppLabels,
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] service account", &serviceAccountResource),
			Entry("[test_id:TODO] service", &serviceResource),
			Entry("[test_id:TODO] deployment", &deploymentResource),
			Entry("[test_id:TODO] config map", &configMapResource),
			Entry("[test_id:TODO] route", &routeResource),
		)
	})

	Context("Resource deletion", func() {
		DescribeTable("recreate after delete", expectRecreateAfterDelete,
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] service account", &serviceAccountResource),
			Entry("[test_id:TODO] service", &serviceResource),
			Entry("[test_id:TODO] deployment", &deploymentResource),
			Entry("[test_id:TODO] config map", &configMapResource),
			Entry("[test_id:TODO] route", &routeResource),
		)
	})

	Context("Resource change", func() {
		DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] service", &serviceResource),
			Entry("[test_id:TODO] deployment", &deploymentResource),
			Entry("[test_id:TODO] config map", &configMapResource),
			Entry("[test_id:TODO] route", &routeResource),
		)

		Context("With pause", func() {
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()
			})

			JustAfterEach(func() {
				unpauseSsp()
			})

			DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				Entry("[test_id:TODO] cluster role", &clusterRoleResource),
				Entry("[test_id:TODO] cluster role binding", &clusterRoleBindingResource),
				Entry("[test_id:TODO] service", &serviceResource),
				Entry("[test_id:TODO] deployment", &deploymentResource),
				Entry("[test_id:TODO] config map", &configMapResource),
				Entry("[test_id:TODO] route", &routeResource),
			)
		})

		DescribeTable("should restore modified app labels", expectAppLabelsRestoreAfterUpdate,
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] service account", &serviceAccountResource),
			Entry("[test_id:TODO] service", &serviceResource),
			Entry("[test_id:TODO] deployment", &deploymentResource),
			Entry("[test_id:TODO] config map", &configMapResource),
			Entry("[test_id:TODO] route", &routeResource),
		)
	})

	Context("Route to access proxy", func() {
		var (
			routeApiUrl string
			httpClient  *http.Client
		)

		BeforeEach(func() {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			httpClient = &http.Client{
				Transport: transport,
			}

			route := &routev1.Route{}
			Expect(apiClient.Get(ctx, routeResource.GetKey(), route)).To(Succeed())
			routeApiUrl = "https://" + route.Spec.Host + "/api/v1alpha1"
		})

		It("[test_id:TODO] should be able to access /token endpoint", func() {
			url, err := url.JoinPath(routeApiUrl, strategy.GetNamespace(), "non-existing-vm", "token")
			Expect(err).ToNot(HaveOccurred())

			response, err := httpClient.Get(url)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = response.Body.Close() }()

			Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))

			body, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(body).To(ContainSubstring("authenticating token cannot be empty"))
		})
	})
})
