package tests

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	vm_console_proxy "kubevirt.io/ssp-operator/internal/operands/vm-console-proxy"
	"kubevirt.io/ssp-operator/tests/env"
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

	BeforeEach(OncePerOrdered, func() {
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

		// Waiting until the proxy deployment is created.
		// This is a workaround, because the above updateSsp() function updates only annotations,
		// which don't update the .metadata.generation field. So the waitUntilDeployed() call
		// below succeeds immediately, and does not wait until proxy resources are created.
		Eventually(func() error {
			return apiClient.Get(ctx, deploymentResource.GetKey(), &apps.Deployment{})
		}, env.ShortTimeout(), time.Second).Should(Succeed())

		waitUntilDeployed()
	})

	AfterEach(OncePerOrdered, func() {
		strategy.RevertToOriginalSspCr()

		// Similar workaround as in BeforeEach().
		originalSspProxyAnnotation := getSsp().Annotations[vm_console_proxy.EnableAnnotation]
		if isEnabled, _ := strconv.ParseBool(originalSspProxyAnnotation); !isEnabled {
			Eventually(func() error {
				deployment := &apps.Deployment{}
				err := apiClient.Get(ctx, deploymentResource.GetKey(), deployment)
				if errors.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return err
				}
				if !deployment.DeletionTimestamp.IsZero() {
					return nil
				}
				return fmt.Errorf("the console proxy deployment is not being deleted")
			}, env.ShortTimeout(), time.Second).Should(Succeed())
		}

		waitUntilDeployed()
	})

	Context("Resource creation", Ordered, func() {
		DescribeTable("created cluster resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, res.GetKey(), resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue(), "owner annotations are missing")
		},
			Entry("[test_id:9888] cluster role", &clusterRoleResource),
			Entry("[test_id:9847] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:9848] service account", &serviceAccountResource),
			Entry("[test_id:9849] service", &serviceResource),
			Entry("[test_id:9850] deployment", &deploymentResource),
			Entry("[test_id:9852] config map", &configMapResource),
			Entry("[test_id:9854] route", &routeResource),
		)

		DescribeTable("should set app labels", expectAppLabels,
			Entry("[test_id:9887] cluster role", &clusterRoleResource),
			Entry("[test_id:9851] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:9853] service account", &serviceAccountResource),
			Entry("[test_id:9855] service", &serviceResource),
			Entry("[test_id:9856] deployment", &deploymentResource),
			Entry("[test_id:9857] config map", &configMapResource),
			Entry("[test_id:9859] route", &routeResource),
		)
	})

	Context("Resource deletion", func() {
		DescribeTable("recreate after delete", expectRecreateAfterDelete,
			Entry("[test_id:9858] cluster role", &clusterRoleResource),
			Entry("[test_id:9860] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:9861] service account", &serviceAccountResource),
			Entry("[test_id:9862] service", &serviceResource),
			Entry("[test_id:9864] deployment", &deploymentResource),
			Entry("[test_id:9866] config map", &configMapResource),
			Entry("[test_id:9867] route", &routeResource),
		)
	})

	Context("Resource change", func() {
		DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			Entry("[test_id:9863] cluster role", &clusterRoleResource),
			Entry("[test_id:9865] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:9869] service", &serviceResource),
			Entry("[test_id:9870] deployment", &deploymentResource),
			Entry("[test_id:9871] config map", &configMapResource),
			Entry("[test_id:9872] route", &routeResource),
		)

		Context("With pause", func() {
			JustAfterEach(func() {
				unpauseSsp()
			})

			DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				Entry("[test_id:9873] cluster role", &clusterRoleResource),
				Entry("[test_id:9874] cluster role binding", &clusterRoleBindingResource),
				Entry("[test_id:9876] service", &serviceResource),
				Entry("[test_id:9877] deployment", &deploymentResource),
				Entry("[test_id:9878] config map", &configMapResource),
				Entry("[test_id:9879] route", &routeResource),
			)
		})

		DescribeTable("should restore modified app labels", expectAppLabelsRestoreAfterUpdate,
			Entry("[test_id:9880] cluster role", &clusterRoleResource),
			Entry("[test_id:9881] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:9882] service account", &serviceAccountResource),
			Entry("[test_id:9886] service", &serviceResource),
			Entry("[test_id:9883] deployment", &deploymentResource),
			Entry("[test_id:9884] config map", &configMapResource),
			Entry("[test_id:9885] route", &routeResource),
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

		It("[test_id:9889] should be able to access /token endpoint", func() {
			url, err := url.JoinPath(routeApiUrl, strategy.GetNamespace(), "non-existing-vm", "token")
			Expect(err).ToNot(HaveOccurred())

			// It may take a moment for the service to be reachable through route
			Eventually(func(g Gomega) {
				response, err := httpClient.Get(url)
				g.Expect(err).ToNot(HaveOccurred())
				defer func() { _ = response.Body.Close() }()

				g.Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))

				body, err := io.ReadAll(response.Body)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(body).To(ContainSubstring("authenticating token cannot be empty"))
			}, env.ShortTimeout(), time.Second).Should(Succeed())
		})
	})
})
