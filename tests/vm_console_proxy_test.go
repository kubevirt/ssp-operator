package tests

import (
	"crypto/tls"
	"io"
	"net/http"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apps "k8s.io/api/apps/v1"
	authnv1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"
	kubevirtcorev1 "kubevirt.io/api/core/v1"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/tests/decorators"
	"kubevirt.io/ssp-operator/tests/env"
)

var _ = Describe("VM Console Proxy Operand", func() {
	var (
		clusterRoleResource        testResource
		clusterRoleBindingResource testResource
		roleBindingResource        testResource
		serviceAccountResource     testResource
		serviceResource            testResource
		deploymentResource         testResource
		configMapResource          testResource
		apiServiceResource         testResource
	)

	BeforeEach(OncePerOrdered, func() {
		strategy.SkipSspUpdateTestsIfNeeded()

		updateSsp(func(foundSsp *ssp.SSP) {
			foundSsp.Spec.TokenGenerationService = &ssp.TokenGenerationService{
				Enabled: true,
			}
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
		roleBindingResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      "kube-system",
			Resource:       &rbac.RoleBinding{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(roleBinding *rbac.RoleBinding) {
				roleBinding.Subjects = nil
			},
			EqualsFunc: func(old *rbac.RoleBinding, new *rbac.RoleBinding) bool {
				return reflect.DeepEqual(old.RoleRef, new.RoleRef) &&
					reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}
		serviceAccountResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetNamespace(),
			Resource:       &core.ServiceAccount{},
			ExpectedLabels: expectedLabels,
		}
		serviceResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetNamespace(),
			Resource:       &core.Service{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(service *core.Service) {
				service.Spec.Ports[0].Port = 1443
				service.Spec.Ports[0].TargetPort = intstr.FromInt32(18768)
			},
			EqualsFunc: func(old *core.Service, new *core.Service) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		deploymentResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetNamespace(),
			Resource:       &apps.Deployment{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(deployment *apps.Deployment) {
				deployment.Spec.Replicas = ptr.To[int32](0)
			},
			EqualsFunc: func(old *apps.Deployment, new *apps.Deployment) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		configMapResource = testResource{
			Name:           "vm-console-proxy",
			Namespace:      strategy.GetNamespace(),
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
		apiServiceResource = testResource{
			Name:           "v1.token.kubevirt.io",
			Resource:       &apiregv1.APIService{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(apiService *apiregv1.APIService) {
				apiService.Spec.VersionPriority = apiService.Spec.VersionPriority + 10
			},
			EqualsFunc: func(old *apiregv1.APIService, new *apiregv1.APIService) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}

		waitUntilDeployed()
	})

	AfterEach(OncePerOrdered, func() {
		strategy.RevertToOriginalSspCr()
	})

	Context("Resource creation", Ordered, func() {
		DescribeTable("created cluster resource", decorators.Conformance, func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, res.GetKey(), resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue(), "owner annotations are missing")
		},
			Entry("[test_id:9888] cluster role", &clusterRoleResource),
			Entry("[test_id:9847] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
		)

		DescribeTable("created resource", decorators.Conformance, func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, res.GetKey(), resource)
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("[test_id:9848] service account", &serviceAccountResource),
			Entry("[test_id:9849] service", &serviceResource),
			Entry("[test_id:9850] deployment", &deploymentResource),
			Entry("[test_id:9852] config map", &configMapResource),
			Entry("[test_id:TODO] API service", &apiServiceResource),
		)

		DescribeTable("should set app labels", expectAppLabels,
			Entry("[test_id:9887] cluster role", &clusterRoleResource),
			Entry("[test_id:9851] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
			Entry("[test_id:9853] service account", &serviceAccountResource),
			Entry("[test_id:9855] service", &serviceResource),
			Entry("[test_id:9856] deployment", &deploymentResource),
			Entry("[test_id:9857] config map", &configMapResource),
			Entry("[test_id:TODO] API service", &apiServiceResource),
		)
	})

	Context("Resource deletion", func() {
		DescribeTable("recreate after delete", decorators.Conformance, expectRecreateAfterDelete,
			Entry("[test_id:9858] cluster role", &clusterRoleResource),
			Entry("[test_id:9860] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
			Entry("[test_id:9861] service account", &serviceAccountResource),
			Entry("[test_id:9862] service", &serviceResource),
			Entry("[test_id:9864] deployment", &deploymentResource),
			Entry("[test_id:9866] config map", &configMapResource),
			Entry("[test_id:TODO] API service", &apiServiceResource),
		)
	})

	Context("Resource change", func() {
		DescribeTable("should restore modified resource", decorators.Conformance, expectRestoreAfterUpdate,
			Entry("[test_id:9863] cluster role", &clusterRoleResource),
			Entry("[test_id:9865] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
			Entry("[test_id:9869] service", &serviceResource),
			Entry("[test_id:9870] deployment", &deploymentResource),
			Entry("[test_id:9871] config map", &configMapResource),
			Entry("[test_id:TODO] API service", &apiServiceResource),
		)

		Context("With pause", func() {
			JustAfterEach(func() {
				unpauseSsp()
			})

			DescribeTable("should restore modified resource with pause", decorators.Conformance, expectRestoreAfterUpdateWithPause,
				Entry("[test_id:9873] cluster role", &clusterRoleResource),
				Entry("[test_id:9874] cluster role binding", &clusterRoleBindingResource),
				Entry("[test_id:TODO] role binding", &roleBindingResource),
				Entry("[test_id:9876] service", &serviceResource),
				Entry("[test_id:9877] deployment", &deploymentResource),
				Entry("[test_id:9878] config map", &configMapResource),
				Entry("[test_id:TODO] API service", &apiServiceResource),
			)
		})

		DescribeTable("should restore modified app labels", expectAppLabelsRestoreAfterUpdate,
			Entry("[test_id:9880] cluster role", &clusterRoleResource),
			Entry("[test_id:9881] cluster role binding", &clusterRoleBindingResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
			Entry("[test_id:9882] service account", &serviceAccountResource),
			Entry("[test_id:9886] service", &serviceResource),
			Entry("[test_id:9883] deployment", &deploymentResource),
			Entry("[test_id:9884] config map", &configMapResource),
			Entry("[test_id:TODO] API service", &apiServiceResource),
		)
	})

	Context("Accessing proxy", func() {
		var (
			httpClient *http.Client
			saToken    string
		)

		BeforeEach(func() {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			httpClient = &http.Client{
				Transport: transport,
			}

			serviceAccount := &core.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "proxy-test-account-",
					Namespace:    strategy.GetNamespace(),
				},
			}
			Expect(apiClient.Create(ctx, serviceAccount)).To(Succeed())
			DeferCleanup(func() {
				err := apiClient.Delete(ctx, serviceAccount)
				if err != nil && !errors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred())
				}
			})

			role := &rbac.Role{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "proxy-test-role-",
					Namespace:    strategy.GetNamespace(),
				},
				Rules: []rbac.PolicyRule{{
					APIGroups: []string{"token.kubevirt.io"},
					Resources: []string{"virtualmachines/vnc"},
					Verbs:     []string{"get"},
				}, {
					APIGroups: []string{kubevirtcorev1.SubresourceGroupName},
					Resources: []string{"virtualmachineinstances/vnc"},
					Verbs:     []string{"get"},
				}},
			}
			Expect(apiClient.Create(ctx, role)).To(Succeed())
			DeferCleanup(func() {
				err := apiClient.Delete(ctx, role)
				if err != nil && !errors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred())
				}
			})

			roleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "proxy-role-binding-",
					Namespace:    strategy.GetNamespace(),
				},
				Subjects: []rbac.Subject{{
					Kind:      "ServiceAccount",
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				}},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "Role",
					Name:     role.Name,
				},
			}
			Expect(apiClient.Create(ctx, roleBinding)).To(Succeed())
			DeferCleanup(func() {
				err := apiClient.Delete(ctx, roleBinding)
				if err != nil && !errors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred())
				}
			})

			tokenRequest, err := coreClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).CreateToken(ctx, serviceAccount.Name, &authnv1.TokenRequest{}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			saToken = tokenRequest.Status.Token
		})

		It("[test_id:TODO] should be able to access /vnc endpoint", decorators.Conformance, func() {
			vmNamespace := strategy.GetNamespace()
			vmName := "non-existing-vm"

			url := apiServerHostname + "/apis/token.kubevirt.io/v1/namespaces/" + vmNamespace + "/virtualmachines/" + vmName + "/vnc"

			// It may take a moment for the service to be reachable
			Eventually(func(g Gomega) {
				request, err := http.NewRequest("GET", url, nil)
				g.Expect(err).ToNot(HaveOccurred())

				request.Header.Set("Authorization", "Bearer "+saToken)

				response, err := httpClient.Do(request)

				g.Expect(err).ToNot(HaveOccurred())
				defer func() { _ = response.Body.Close() }()

				g.Expect(response.StatusCode).To(Equal(http.StatusNotFound))

				body, err := io.ReadAll(response.Body)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(body).To(ContainSubstring("VirtualMachine does not exist:"))
			}, env.ShortTimeout(), time.Second).Should(Succeed())
		})
	})
})
