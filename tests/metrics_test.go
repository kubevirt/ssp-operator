package tests

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	"kubevirt.io/ssp-operator/tests/env"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apps "k8s.io/api/apps/v1"
	rbac "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
)

func mergeMaps(maps ...map[string]string) map[string]string {
	targetMap := make(map[string]string)
	for _, sourceMap := range maps {
		for k, v := range sourceMap {
			targetMap[k] = v
		}
	}
	return targetMap
}

func getDeployment(name string, namespace string) *apps.Deployment {
	deployment := &apps.Deployment{}
	err := apiClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployment)
	Expect(err).ToNot(HaveOccurred())
	return deployment
}

var _ = Describe("Metrics", func() {
	var (
		prometheusRuleRes         testResource
		serviceMonitorRes         testResource
		rbacClusterRoleRes        testResource
		rbacClusterRoleBindingRes testResource
	)

	BeforeEach(func() {
		expectedLabels := expectedLabelsFor("metrics", common.AppComponentMonitoring)

		serviceMonitorRes = testResource{
			Name:           metrics.PrometheusRuleName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &promv1.ServiceMonitor{},
			ExpectedLabels: mergeMaps(expectedLabels, metrics.ServiceMonitorLabels()),
			UpdateFunc: func(ServiceMonitor *promv1.ServiceMonitor) {
				ServiceMonitor.Spec.Selector = v1.LabelSelector{}
				ServiceMonitor.Spec.NamespaceSelector = promv1.NamespaceSelector{}
			},
			EqualsFunc: func(old *promv1.ServiceMonitor, new *promv1.ServiceMonitor) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		rbacClusterRoleRes = testResource{
			Name:           metrics.PrometheusClusterRoleName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &rbac.ClusterRole{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(role *rbac.ClusterRole) {
				role.Rules[0].Verbs = []string{"watch"}
			},
			EqualsFunc: func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		rbacClusterRoleBindingRes = testResource{
			Name:           metrics.PrometheusClusterRoleName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &rbac.ClusterRoleBinding{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(roleBinding *rbac.ClusterRoleBinding) {
				roleBinding.Subjects = nil
			},
			EqualsFunc: func(old *rbac.ClusterRoleBinding, new *rbac.ClusterRoleBinding) bool {
				return reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}

		prometheusRuleRes = testResource{
			Name:           metrics.PrometheusRuleName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &promv1.PrometheusRule{},
			ExpectedLabels: expectedLabelsFor("metrics", common.AppComponentMonitoring),
			UpdateFunc: func(rule *promv1.PrometheusRule) {
				rule.Spec.Groups[0].Name = "changed-name"
				rule.Spec.Groups[0].Rules = []promv1.Rule{}
			},
			EqualsFunc: func(old, new *promv1.PrometheusRule) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}

		waitUntilDeployed()
	})

	Context("resource creation", func() {
		DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, res.GetKey(), res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("[test_id:8346] service monitor", &serviceMonitorRes),
			Entry("[test_id:8347] role", &rbacClusterRoleRes),
			Entry("[test_id:8345] role binding", &rbacClusterRoleBindingRes),
			Entry("[test_id:4665] prometheus rules", &prometheusRuleRes),
		)

		DescribeTable("should set app labels", expectAppLabels,
			Entry("[test_id:8348] service monitor", &serviceMonitorRes),
			Entry("[test_id:8349] role", &rbacClusterRoleRes),
			Entry("[test_id:8350] role binding", &rbacClusterRoleBindingRes),
			Entry("[test_id:5790] prometheus rules", &prometheusRuleRes),
		)
	})

	Context("resource deletion", func() {
		DescribeTable("recreate after delete", expectRecreateAfterDelete,
			Entry("[test_id:8351] service monitor", &serviceMonitorRes),
			Entry("[test_id:8352] role", &rbacClusterRoleRes),
			Entry("[test_id:8355] role binding", &rbacClusterRoleBindingRes),
			Entry("[test_id:6055] prometheus rules", &prometheusRuleRes),
		)
	})

	Context("resource change", func() {
		DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			Entry("[test_id:8356] service monitor", &serviceMonitorRes),
			Entry("[test_id:8353] role", &rbacClusterRoleRes),
			Entry("[test_id:8354] role binding", &rbacClusterRoleBindingRes),
			Entry("[test_id:4666] prometheus rules", &prometheusRuleRes),
		)

		Context("with pause", func() {
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()
			})

			JustAfterEach(func() {
				unpauseSsp()
			})

			DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				Entry("[test_id:8357] service monitor", &serviceMonitorRes),
				Entry("[test_id:8358] role", &rbacClusterRoleRes),
				Entry("[test_id:8361] role binding", &rbacClusterRoleBindingRes),
				Entry("[test_id:5397] prometheus rules", &prometheusRuleRes),
			)
		})

		DescribeTable("should restore modified app labels", expectAppLabelsRestoreAfterUpdate,
			Entry("[test_id:8362] service monitor", &serviceMonitorRes),
			Entry("[test_id:8359] role", &rbacClusterRoleRes),
			Entry("[test_id:8360] role binding", &rbacClusterRoleBindingRes),
			Entry("[test_id:5790] prometheus rules", &prometheusRuleRes),
		)
	})

	Context("SSP metrics", func() {
		var originalVersion string

		waitForVersionUpdate := func(version string) {
			Eventually(func() error {
				rsList := &apps.ReplicaSetList{}
				err := apiClient.List(ctx, rsList, client.InNamespace(sspDeploymentNamespace),
					client.MatchingLabels{"name": "ssp-operator"})
				Expect(err).ToNot(HaveOccurred())

				for _, rs := range rsList.Items {
					for _, env := range rs.Spec.Template.Spec.Containers[0].Env {
						if env.Name == common.OperatorVersionKey &&
							env.Value == version &&
							*rs.Spec.Replicas > 0 &&
							rs.Status.ReadyReplicas == *rs.Spec.Replicas {
							return nil
						}
					}
				}

				return fmt.Errorf("operator version not yet updated")
			}, env.ShortTimeout(), time.Second).Should(Succeed())
		}

		updateVersion := func(version string) {
			Eventually(func() error {
				deployment := getDeployment(sspDeploymentName, sspDeploymentNamespace)

				envs := deployment.Spec.Template.Spec.Containers[0].Env

				for i, env := range envs {
					if env.Name == common.OperatorVersionKey {
						envs[i].Value = version
						break
					}
				}

				return apiClient.Update(ctx, deployment)
			}, env.ShortTimeout(), time.Second).ShouldNot(HaveOccurred())

			waitForVersionUpdate(version)
		}

		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			By("saving original version")
			originalVersion = ""
			deployment := getDeployment(sspDeploymentName, sspDeploymentNamespace)
			envs := deployment.Spec.Template.Spec.Containers[0].Env
			for _, env := range envs {
				if env.Name == common.OperatorVersionKey {
					originalVersion = env.Value
				}
			}

			waitUntilDeployed()
		})

		AfterEach(func() {
			updateVersion(originalVersion)
			strategy.RevertToOriginalSspCr()
		})

		It("[test_id:TODO]should not increment total_restored_common_templates during upgrades", func() {
			testTemplate := createTestTemplate()

			pauseSsp()
			version := "test-" + rand.String(5)
			updateVersion(version)

			template := &templatev1.Template{}
			Expect(apiClient.Get(ctx, testTemplate.GetKey(), template)).To(Succeed())
			testTemplate.Update(template)
			Expect(apiClient.Update(ctx, template)).To(Succeed())

			unpauseSsp()
			waitUntilDeployed()

			newRestoredCount := totalRestoredTemplatesCount()
			Expect(newRestoredCount).To(BeZero())
		})
	})

	Context("SSP alerts rules", func() {
		var promRule promv1.PrometheusRule

		BeforeEach(func() {
			Expect(apiClient.Get(ctx, prometheusRuleRes.GetKey(), &promRule)).To(Succeed())
		})

		It("[test_id:7851]should have all the required annotations", func() {
			for _, group := range promRule.Spec.Groups {
				for _, rule := range group.Rules {
					if rule.Alert != "" && rule.Alert != metrics.Rhel6AlertName {
						Expect(rule.Annotations).To(HaveKeyWithValue("summary", Not(BeEmpty())),
							fmt.Sprintf("%s summary is missing or empty", rule.Alert))
						Expect(rule.Annotations).To(HaveKeyWithValue("runbook_url", Not(BeEmpty())),
							fmt.Sprintf("%s runbook_url is missing or empty", rule.Alert))
						resp, err := http.Head(rule.Annotations["runbook_url"])
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s runbook is not available", rule.Alert))
						Expect(resp.StatusCode).Should(Equal(http.StatusOK), fmt.Sprintf("%s runbook is not available", rule.Alert))
					}
				}
			}
		})

		It("[test_id:8955]should have all the required labels", func() {
			for _, group := range promRule.Spec.Groups {
				for _, rule := range group.Rules {
					if rule.Alert != "" {
						Expect(rule.Labels).To(HaveKeyWithValue("severity", BeElementOf("info", "warning", "critical")),
							fmt.Sprintf("%s severity label is missing or not valid", rule.Alert))
						Expect(rule.Labels).To(HaveKeyWithValue("operator_health_impact", BeElementOf("none", "warning", "critical")),
							fmt.Sprintf("%s operator_health_impact label is missing or not valid", rule.Alert))
						Expect(rule.Labels).To(HaveKeyWithValue("kubernetes_operator_part_of", "kubevirt"),
							fmt.Sprintf("%s kubernetes_operator_part_of label is missing or not valid", rule.Alert))
						Expect(rule.Labels).To(HaveKeyWithValue("kubernetes_operator_component", "ssp-operator"),
							fmt.Sprintf("%s kubernetes_operator_component label is missing or not valid", rule.Alert))
					}
				}
			}
		})
	})
})
