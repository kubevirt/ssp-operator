package tests

import (
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/extensions/table"
	authv1 "k8s.io/api/authorization/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	data_sources "kubevirt.io/ssp-operator/internal/operands/data-sources"
)

var _ = Describe("DataSources", func() {
	var (
		viewRole        testResource
		viewRoleBinding testResource
		editClusterRole testResource
		goldenImageNS   testResource
	)

	BeforeEach(func() {
		expectedLabels := expectedLabelsFor("data-sources", common.AppComponentTemplating)
		viewRole = testResource{
			Name:           data_sources.ViewRoleName,
			Namespace:      ssp.GoldenImagesNSname,
			Resource:       &rbac.Role{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(role *rbac.Role) {
				role.Rules = []rbac.PolicyRule{}
			},
			EqualsFunc: func(old *rbac.Role, new *rbac.Role) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		viewRoleBinding = testResource{
			Name:           data_sources.ViewRoleName,
			Namespace:      ssp.GoldenImagesNSname,
			Resource:       &rbac.RoleBinding{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(roleBinding *rbac.RoleBinding) {
				roleBinding.Subjects = nil
			},
			EqualsFunc: func(old *rbac.RoleBinding, new *rbac.RoleBinding) bool {
				return reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}
		editClusterRole = testResource{
			Name:           data_sources.EditClusterRoleName,
			Resource:       &rbac.ClusterRole{},
			ExpectedLabels: expectedLabels,
			Namespace:      "",
			UpdateFunc: func(role *rbac.ClusterRole) {
				role.Rules[0].Verbs = []string{"watch"}
			},
			EqualsFunc: func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		goldenImageNS = testResource{
			Name:           ssp.GoldenImagesNSname,
			Resource:       &core.Namespace{},
			ExpectedLabels: expectedLabels,
			Namespace:      "",
		}

		waitUntilDeployed()
	})

	Context("resource creation", func() {
		table.DescribeTable("created cluster resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, res.GetKey(), resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue(), "Missing owner annotations")
		},
			table.Entry("[test_id:4584]edit role", &editClusterRole),
			table.Entry("[test_id:4494]golden images namespace", &goldenImageNS),
		)

		table.DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, res.GetKey(), res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:4777]view role", &viewRole),
			table.Entry("[test_id:4772]view role binding", &viewRoleBinding),
		)

		table.DescribeTable("should set app labels", expectAppLabels,
			table.Entry("[test_id:6215] edit role", &editClusterRole),
			table.Entry("[test_id:6216] golden images namespace", &goldenImageNS),
			table.Entry("[test_id:6217] view role", &viewRole),
			table.Entry("[test_id:6218] view role binding", &viewRoleBinding),
		)
	})

	Context("resource change", func() {
		table.DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			table.Entry("[test_id:5315]edit cluster role", &editClusterRole),
			table.Entry("[test_id:5316]view role", &viewRole),
			table.Entry("[test_id:5317]view role binding", &viewRoleBinding),
		)

		Context("with pause", func() {
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()
			})

			JustAfterEach(func() {
				unpauseSsp()
			})

			table.DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				table.Entry("[test_id:5388]view role", &viewRole),
				table.Entry("[test_id:5389]view role binding", &viewRoleBinding),
				table.Entry("[test_id:5393]edit cluster role", &editClusterRole),
			)
		})

		table.DescribeTable("should restore app labels", expectAppLabelsRestoreAfterUpdate,
			table.Entry("[test_id:6210] edit role", &editClusterRole),
			table.Entry("[test_id:6211] golden images namespace", &goldenImageNS),
			table.Entry("[test_id:6212] view role", &viewRole),
			table.Entry("[test_id:6213] view role binding", &viewRoleBinding),
		)
	})

	Context("resource deletion", func() {
		table.DescribeTable("recreate after delete", expectRecreateAfterDelete,
			table.Entry("[test_id:4773]view role", &viewRole),
			table.Entry("[test_id:4842]view role binding", &viewRoleBinding),
			table.Entry("[test_id:4771]edit cluster role", &editClusterRole),
			table.Entry("[test_id:4770]golden image NS", &goldenImageNS),
		)
	})

	Context("rbac", func() {
		Context("os-images", func() {
			var (
				regularSA         *core.ServiceAccount
				regularSAFullName string
				sasGroup          = []string{"system:serviceaccounts"}
			)

			BeforeEach(func() {
				regularSA = &core.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "regular-sa-",
						Namespace:    strategy.GetNamespace(),
					},
				}
				Expect(apiClient.Create(ctx, regularSA)).To(Succeed(), "creation of regular service account failed")

				regularSAFullName = fmt.Sprintf("system:serviceaccount:%s:%s", regularSA.GetNamespace(), regularSA.GetName())
			})

			AfterEach(func() {
				Expect(apiClient.Delete(ctx, regularSA)).NotTo(HaveOccurred())
			})

			table.DescribeTable("regular service account namespace RBAC", expectUserCan,
				table.Entry("[test_id:6069] should be able to 'get' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "namespaces",
						},
					}),
				table.Entry("[test_id:6070] should be able to 'list' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "namespaces",
						},
					}),
				table.Entry("[test_id:6071] should be able to 'watch' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "watch",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "namespaces",
						},
					}))

			table.DescribeTable("regular service account DV RBAC", expectUserCan,
				table.Entry("[test_id:6072] should be able to 'get' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
				table.Entry("[test_id:6073] should be able to 'list' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
				table.Entry("[test_id:6074] should be able to 'watch' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "watch",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
				table.Entry("[test_id:5005]: ServiceAccounts with only view role can create dv/source",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:        "create",
							Namespace:   ssp.GoldenImagesNSname,
							Group:       cdiv1beta1.SchemeGroupVersion.Group,
							Version:     cdiv1beta1.SchemeGroupVersion.Version,
							Resource:    "datavolumes",
							Subresource: "source",
						},
					}),
			)

			table.DescribeTable("regular service account DV RBAC", expectUserCannot,
				table.Entry("[test_id:4873]: ServiceAccounts with only view role cannot delete DVs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "delete",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
				table.Entry("[test_id:4874]: ServiceAccounts with only view role cannot create DVs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
			)
			table.DescribeTable("regular service account PVC RBAC", expectUserCan,
				table.Entry("[test_id:4775]: ServiceAccounts with view role can view PVCs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "persistentvolumeclaims",
						},
					}))
			table.DescribeTable("regular service account RBAC", expectUserCannot,
				table.Entry("[test_id:4776]: ServiceAccounts with only view role cannot create PVCs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "persistentvolumeclaims",
						},
					}),
				table.Entry("[test_id:4846]: ServiceAccounts with only view role cannot delete PVCs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "delete",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "persistentvolumeclaims",
						},
					}),
				table.Entry("[test_id:4879]: ServiceAccounts with only view role cannot create any other resources other than the ones listed in the View role",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "pods",
						},
					}),
			)
			Context("With Edit permission", func() {
				var (
					privilegedSA         *core.ServiceAccount
					privilegedSAFullName string

					editObj *rbac.RoleBinding
				)
				BeforeEach(func() {
					privilegedSA = &core.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "privileged-sa-",
							Namespace:    strategy.GetNamespace(),
						},
					}

					Expect(apiClient.Create(ctx, privilegedSA)).To(Succeed(), "creation of regular service account failed")
					privilegedSAFullName = fmt.Sprintf("system:serviceaccount:%s:%s", privilegedSA.GetNamespace(), privilegedSA.GetName())

					editObj = &rbac.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "test-edit-",
							Namespace:    ssp.GoldenImagesNSname,
						},
						Subjects: []rbac.Subject{{
							Kind:      "ServiceAccount",
							Name:      privilegedSA.GetName(),
							Namespace: privilegedSA.GetNamespace(),
						}},
						RoleRef: rbac.RoleRef{
							Kind:     "ClusterRole",
							Name:     data_sources.EditClusterRoleName,
							APIGroup: rbac.GroupName,
						},
					}
					Expect(apiClient.Create(ctx, editObj)).ToNot(HaveOccurred(), "Failed to create RoleBinding")
				})
				AfterEach(func() {
					Expect(apiClient.Delete(ctx, editObj)).ToNot(HaveOccurred())
					Expect(apiClient.Delete(ctx, privilegedSA)).NotTo(HaveOccurred())
				})
				table.DescribeTable("should verify resource permissions", func(sars *authv1.SubjectAccessReviewSpec) {
					// Because privilegedSAFullName is filled after test Tree generation
					sars.User = privilegedSAFullName
					expectUserCan(sars)
				},
					table.Entry("[test_id:4774]: ServiceAcounts with edit role can create PVCs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: ssp.GoldenImagesNSname,
								Version:   core.SchemeGroupVersion.Version,
								Resource:  "persistentvolumeclaims",
							},
						}),
					table.Entry("[test_id:4845]: ServiceAcounts with edit role can delete PVCs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: ssp.GoldenImagesNSname,
								Version:   core.SchemeGroupVersion.Version,
								Resource:  "persistentvolumeclaims",
							},
						}),
					table.Entry("[test_id:4877]: ServiceAccounts with edit role can view DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "get",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),
					table.Entry("[test_id:4872]: ServiceAccounts with edit role can create DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),
					table.Entry("[test_id:4876]: ServiceAccounts with edit role can delete DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),
				)
				It("[test_id:4878]should not create any other resurces than the ones listed in the Edit Cluster role", func() {
					sars := &authv1.SubjectAccessReviewSpec{
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "pods",
						},
					}
					sars.User = privilegedSAFullName
					expectUserCannot(sars)
				})
			})
		})
	})
})
