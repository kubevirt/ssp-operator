package tests

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/extensions/table"
	authv1 "k8s.io/api/authorization/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal"
	"kubevirt.io/ssp-operator/internal/common"
	data_sources "kubevirt.io/ssp-operator/internal/operands/data-sources"
)

var _ = Describe("DataSources", func() {
	// The name must be one of the DataSources needed by common templates
	const dataSourceName = "fedora"

	const cdiLabelPrefix = "cdi.kubevirt.io"
	const cdiLabel = cdiLabelPrefix + "/dataImportCron"
	const cdiCleanupLabel = cdiLabel + ".cleanup"

	var (
		expectedLabels map[string]string

		viewRole        testResource
		viewRoleBinding testResource
		editClusterRole testResource
		goldenImageNS   testResource
		dataSource      testResource
	)

	BeforeEach(func() {
		expectedLabels = expectedLabelsFor("data-sources", common.AppComponentTemplating)
		viewRole = testResource{
			Name:           data_sources.ViewRoleName,
			Namespace:      internal.GoldenImagesNamespace,
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
			Namespace:      internal.GoldenImagesNamespace,
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
			Name:           internal.GoldenImagesNamespace,
			Resource:       &core.Namespace{},
			ExpectedLabels: expectedLabels,
			Namespace:      "",
		}
		dataSource = testResource{
			Name:           dataSourceName,
			Namespace:      internal.GoldenImagesNamespace,
			Resource:       &cdiv1beta1.DataSource{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(ds *cdiv1beta1.DataSource) {
				ds.Spec.Source.PVC.Name = "testing-non-existing-name"
			},
			EqualsFunc: func(old, new *cdiv1beta1.DataSource) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
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
							Namespace: internal.GoldenImagesNamespace,
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
							Namespace: internal.GoldenImagesNamespace,
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
							Namespace: internal.GoldenImagesNamespace,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "namespaces",
						},
					}))

			table.DescribeTable("regular service account DV RBAC allowed", expectUserCan,
				table.Entry("[test_id:6072] should be able to 'get' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: internal.GoldenImagesNamespace,
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
							Namespace: internal.GoldenImagesNamespace,
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
							Namespace: internal.GoldenImagesNamespace,
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
							Namespace:   internal.GoldenImagesNamespace,
							Group:       cdiv1beta1.SchemeGroupVersion.Group,
							Version:     cdiv1beta1.SchemeGroupVersion.Version,
							Resource:    "datavolumes",
							Subresource: "source",
						},
					}),
			)

			table.DescribeTable("regular service account DV RBAC denied", expectUserCannot,
				table.Entry("[test_id:4873]: ServiceAccounts with only view role cannot delete DVs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "delete",
							Namespace: internal.GoldenImagesNamespace,
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
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
			)

			table.DescribeTable("regular service account PVC RBAC allowed", expectUserCan,
				table.Entry("[test_id:4775]: ServiceAccounts with view role can view PVCs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: internal.GoldenImagesNamespace,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "persistentvolumeclaims",
						},
					}))

			table.DescribeTable("regular service account PVC RBAC denied", expectUserCannot,
				table.Entry("[test_id:4776]: ServiceAccounts with only view role cannot create PVCs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: internal.GoldenImagesNamespace,
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
							Namespace: internal.GoldenImagesNamespace,
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
							Namespace: internal.GoldenImagesNamespace,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "pods",
						},
					}),
			)

			table.DescribeTable("regular service account DataSource RBAC allowed", expectUserCan,
				table.Entry("[test_id:7466] should be able to 'get' datasources",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datasources",
						},
					}),
				table.Entry("[test_id:7468] should be able to 'list' datasources",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datasources",
						},
					}),
				table.Entry("[test_id:7462] should be able to 'watch' datasources",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "watch",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datasources",
						},
					}),
			)

			table.DescribeTable("regular service account DataSource RBAC denied", expectUserCannot,
				table.Entry("[test_id:7464]: ServiceAccounts with only view role cannot delete DataSources",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "delete",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datasources",
						},
					}),
				table.Entry("[test_id:7450]: ServiceAccounts with only view role cannot create DataSources",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datasources",
						},
					}),
			)

			table.DescribeTable("regular service account DataImportCron RBAC allowed", expectUserCan,
				table.Entry("[test_id:7460] should be able to 'get' DataImportCrons",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "dataimportcrons",
						},
					}),
				table.Entry("[test_id:7461] should be able to 'list' DataImportCrons",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "dataimportcrons",
						},
					}),
				table.Entry("[test_id:7459] should be able to 'watch' DataImportCrons",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "watch",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "dataimportcrons",
						},
					}),
			)

			table.DescribeTable("regular service account DataImportCron RBAC denied", expectUserCannot,
				table.Entry("[test_id:7456]: ServiceAccounts with only view role cannot delete DataImportCrons",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "delete",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "dataimportcrons",
						},
					}),
				table.Entry("[test_id:7454]: ServiceAccounts with only view role cannot create DataImportCrons",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: internal.GoldenImagesNamespace,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "dataimportcrons",
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
							Namespace:    internal.GoldenImagesNamespace,
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
								Namespace: internal.GoldenImagesNamespace,
								Version:   core.SchemeGroupVersion.Version,
								Resource:  "persistentvolumeclaims",
							},
						}),
					table.Entry("[test_id:4845]: ServiceAcounts with edit role can delete PVCs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: internal.GoldenImagesNamespace,
								Version:   core.SchemeGroupVersion.Version,
								Resource:  "persistentvolumeclaims",
							},
						}),
					table.Entry("[test_id:4877]: ServiceAccounts with edit role can view DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "get",
								Namespace: internal.GoldenImagesNamespace,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),
					table.Entry("[test_id:4872]: ServiceAccounts with edit role can create DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: internal.GoldenImagesNamespace,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),
					table.Entry("[test_id:4876]: ServiceAccounts with edit role can delete DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: internal.GoldenImagesNamespace,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),

					table.Entry("[test_id:7452]: ServiceAccounts with edit role can create DataSources",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: internal.GoldenImagesNamespace,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datasources",
							},
						}),
					table.Entry("[test_id:7451]: ServiceAccounts with edit role can delete DataSources",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: internal.GoldenImagesNamespace,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datasources",
							},
						}),

					table.Entry("[test_id:7449]: ServiceAccounts with edit role can create DataImportCrons",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: internal.GoldenImagesNamespace,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "dataimportcrons",
							},
						}),
					table.Entry("[test_id:7448]: ServiceAccounts with edit role can delete DataImportCrons",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: internal.GoldenImagesNamespace,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "dataimportcrons",
							},
						}),
				)
				It("[test_id:4878]should not create any other resurces than the ones listed in the Edit Cluster role", func() {
					sars := &authv1.SubjectAccessReviewSpec{
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: internal.GoldenImagesNamespace,
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

	Context("without DataImportCron templates", func() {
		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			// Removing any existing DataImportCron templates.
			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.CommonTemplates.DataImportCronTemplates = nil
			})
			waitUntilDeployed()
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		It("[test_id:8105] should create DataSource", func() {
			Expect(apiClient.Get(ctx, dataSource.GetKey(), dataSource.NewResource())).To(Succeed())
		})

		It("[test_id:8106] should set app labels on DataSource", func() {
			expectAppLabels(&dataSource)
		})

		It("[test_id:8107] should restore modified DataSource", func() {
			expectRestoreAfterUpdate(&dataSource)
		})

		Context("with pause", func() {
			JustAfterEach(func() {
				unpauseSsp()
			})

			It("[test_id:8108] should restore modified DataSource with pause", func() {
				expectRestoreAfterUpdateWithPause(&dataSource)
			})
		})

		It("[test_id:8115] should restore app labels on DataSource", func() {
			expectAppLabelsRestoreAfterUpdate(&dataSource)
		})

		It("[test_id:8109] should recreate DataSource after delete", func() {
			expectRecreateAfterDelete(&dataSource)
		})

		Context("with added CDI label", func() {
			BeforeEach(func() {
				Eventually(func() error {
					ds := &cdiv1beta1.DataSource{}
					err := apiClient.Get(ctx, dataSource.GetKey(), ds)
					if err != nil {
						return err
					}

					if ds.GetLabels() == nil {
						ds.SetLabels(make(map[string]string))
					}
					ds.GetLabels()[cdiLabel] = "test-value"

					return apiClient.Update(ctx, ds)
				}, shortTimeout, time.Second).Should(Succeed())
			})

			AfterEach(func() {
				Eventually(func() error {
					ds := &cdiv1beta1.DataSource{}
					Expect(apiClient.Get(ctx, dataSource.GetKey(), ds))
					delete(ds.GetLabels(), cdiLabel)
					return apiClient.Update(ctx, ds)
				}, shortTimeout, time.Second).Should(Succeed())
			})

			It("[test_id:8294] should remove CDI label from DataSource", func() {
				// Wait until it is removed
				Eventually(func() (bool, error) {
					ds := &cdiv1beta1.DataSource{}
					err := apiClient.Get(ctx, dataSource.GetKey(), ds)
					if err != nil {
						return false, err
					}

					_, labelExists := ds.GetLabels()[cdiLabel]
					return labelExists, nil
				}, shortTimeout, time.Second).Should(BeFalse(), "Label '"+cdiLabel+"' should not be on DataSource")
			})
		})

	})

	Context("with DataImportCron template", func() {
		const cronSchedule = "* * * * *"

		const cronName = "test-data-import-cron"

		var (
			registryURL       = "docker://quay.io/kubevirt/cirros-container-disk-demo"
			pullMethod        = cdiv1beta1.RegistryPullNode
			commonAnnotations = map[string]string{
				"cdi.kubevirt.io/storage.bind.immediate.requested": "true",
			}

			cronTemplate   ssp.DataImportCronTemplate
			dataImportCron testResource
		)

		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			// Removing any existing DataImportCron templates.
			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.CommonTemplates.DataImportCronTemplates = nil
			})
			waitUntilDeployed()

			retentionPolicyNone := cdiv1beta1.DataImportCronRetainNone
			cronTemplate = ssp.DataImportCronTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:        cronName,
					Annotations: commonAnnotations,
				},
				Spec: cdiv1beta1.DataImportCronSpec{
					Schedule:          cronSchedule,
					ManagedDataSource: dataSourceName,
					RetentionPolicy:   &retentionPolicyNone,
					Template: cdiv1beta1.DataVolume{
						Spec: cdiv1beta1.DataVolumeSpec{
							Source: &cdiv1beta1.DataVolumeSource{
								Registry: &cdiv1beta1.DataVolumeSourceRegistry{
									URL:        &registryURL,
									PullMethod: &pullMethod,
								},
							},
							Storage: &cdiv1beta1.StorageSpec{
								Resources: core.ResourceRequirements{
									Requests: core.ResourceList{
										core.ResourceStorage: resource.MustParse("128Mi"),
									},
								},
							},
						},
					},
				},
			}

			dataImportCron = testResource{
				Name:           cronTemplate.Name,
				Namespace:      internal.GoldenImagesNamespace,
				Resource:       &cdiv1beta1.DataImportCron{},
				ExpectedLabels: expectedLabels,
			}
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		Context("without existing PVC", func() {
			BeforeEach(func() {
				updateSsp(func(foundSsp *ssp.SSP) {
					foundSsp.Spec.CommonTemplates.DataImportCronTemplates = append(foundSsp.Spec.CommonTemplates.DataImportCronTemplates,
						cronTemplate,
					)
				})

				waitUntilDeployed()
			})

			It("[test_id:7469] should create DataImportCron in golden images namespace", func() {
				Expect(apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())).To(Succeed(), "custom DataImportCron created")
			})

			It("[test_id:7467] should set app labels on DataImportCron", func() {
				expectAppLabels(&dataImportCron)
			})

			It("[test_id:7458] should recreate DataImportCron after delete in golden images namespace", func() {
				expectRecreateAfterDelete(&dataImportCron)
			})

			It("[test_id:7712] should update DataImportCron if updated in SSP CR", func() {
				updateSsp(func(foundSsp *ssp.SSP) {
					foundSsp.Spec.CommonTemplates.DataImportCronTemplates[0].
						Spec.Template.Spec.Storage.Resources.Requests[core.ResourceStorage] = resource.MustParse("32Mi")
				})

				waitUntilDeployed()

				cron := &cdiv1beta1.DataImportCron{}
				Expect(apiClient.Get(ctx, dataImportCron.GetKey(), cron)).To(Succeed())
				Expect(cron.Spec.Template.Spec.Storage.Resources.Requests).To(HaveKeyWithValue(core.ResourceStorage, resource.MustParse("32Mi")))
			})

			It("[test_id:7455] should remove DataImportCron in golden images namespace if removed from SSP CR", func() {
				updateSsp(func(foundSsp *ssp.SSP) {
					foundSsp.Spec.CommonTemplates.DataImportCronTemplates = nil
				})

				waitUntilDeployed()

				cron := &cdiv1beta1.DataImportCron{}
				err := apiClient.Get(ctx, dataImportCron.GetKey(), cron)
				if err != nil {
					Expect(errors.IsNotFound(err)).To(BeTrue(), "Expected error to be: IsNotFound")
				} else {
					Expect(cron.GetDeletionTimestamp().IsZero()).To(BeFalse(), "DataImportCron is not being deleted")
				}
			})

			It("[test_id:8295] DataSource should have CDI label", func() {
				Eventually(func() map[string]string {
					ds := &cdiv1beta1.DataSource{}
					Expect(apiClient.Get(ctx, dataSource.GetKey(), ds))
					return ds.GetLabels()
				}, shortTimeout, time.Second).Should(HaveKeyWithValue(cdiLabel, cronName))
			})

			It("[test_id:8112] should restore DataSource if DataImportCron removed from SSP CR", func() {
				// Wait until DataImportCron imports PVC and changes data source
				Eventually(func() bool {
					cron := &cdiv1beta1.DataImportCron{}
					Expect(apiClient.Get(ctx, dataImportCron.GetKey(), cron)).To(Succeed())

					return cron.Status.LastImportTimestamp.IsZero()
				}, timeout, time.Second).Should(BeFalse(), "DataImportCron did not finish importing.")

				managedDataSource := &cdiv1beta1.DataSource{}
				Expect(apiClient.Get(ctx, dataSource.GetKey(), managedDataSource)).To(Succeed())

				// Remove the DataImportCron
				updateSsp(func(foundSsp *ssp.SSP) {
					foundSsp.Spec.CommonTemplates.DataImportCronTemplates = nil
				})
				waitUntilDeployed()

				// Check if the DataSource has been reverted
				revertedDataSource := &cdiv1beta1.DataSource{}
				Expect(apiClient.Get(ctx, dataSource.GetKey(), revertedDataSource)).To(Succeed())

				Expect(revertedDataSource).ToNot(EqualResource(&dataSource, managedDataSource))

				// Delete the DataSource and let the operator recreate it
				Expect(apiClient.Delete(ctx, revertedDataSource.DeepCopy())).To(Succeed())

				recreatedDataSource := &cdiv1beta1.DataSource{}
				Eventually(func() error {
					return apiClient.Get(ctx, dataSource.GetKey(), recreatedDataSource)
				}, shortTimeout, time.Second).Should(Succeed())

				Expect(revertedDataSource).To(EqualResource(&dataSource, recreatedDataSource))
			})

			It("[test_id:8296] should restore CDI label on DataSource, if user removes it", func() {
				Eventually(func() error {
					ds := &cdiv1beta1.DataSource{}
					Expect(apiClient.Get(ctx, dataSource.GetKey(), ds))
					delete(ds.GetLabels(), cdiLabel)
					return apiClient.Update(ctx, ds)
				}, shortTimeout, time.Second).Should(Succeed())

				// Eventually the label should be added back
				Eventually(func() map[string]string {
					ds := &cdiv1beta1.DataSource{}
					Expect(apiClient.Get(ctx, dataSource.GetKey(), ds))
					return ds.GetLabels()
				}, shortTimeout, time.Second).Should(HaveKeyWithValue(cdiLabel, cronName))
			})
		})

		Context("without existing PVC and custom namespace", func() {
			var (
				customNamespace core.Namespace
			)

			BeforeEach(func() {
				customNamespace = core.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom-namespace",
					},
				}
				Expect(apiClient.Create(ctx, &customNamespace)).To(Succeed())

				cronTemplate.Namespace = customNamespace.Name
				dataImportCron.Namespace = customNamespace.Name
				updateSsp(func(foundSsp *ssp.SSP) {
					foundSsp.Spec.CommonTemplates.DataImportCronTemplates = append(foundSsp.Spec.CommonTemplates.DataImportCronTemplates,
						cronTemplate,
					)
				})

				waitUntilDeployed()
			})

			AfterEach(func() {
				Expect(apiClient.Delete(ctx, &customNamespace)).To(Succeed())
				waitForDeletion(client.ObjectKeyFromObject(&customNamespace), &core.Namespace{})
			})

			It("[test_id:????] should create DataImportCron ", func() {
				Expect(apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())).To(Succeed(), "custom DataImportCron created")
			})

			It("[test_id:????] should recreate DataImportCron after delete", func() {
				expectRecreateAfterDelete(&dataImportCron)
			})

			It("[test_id:????] should remove DataImportCron if removed from SSP CR", func() {
				updateSsp(func(foundSsp *ssp.SSP) {
					foundSsp.Spec.CommonTemplates.DataImportCronTemplates = nil
				})

				waitUntilDeployed()

				cron := &cdiv1beta1.DataImportCron{}
				err := apiClient.Get(ctx, dataImportCron.GetKey(), cron)
				if err != nil {
					Expect(errors.IsNotFound(err)).To(BeTrue(), "Expected error to be: IsNotFound")
				} else {
					Expect(cron.GetDeletionTimestamp().IsZero()).To(BeFalse(), "DataImportCron is not being deleted")
				}
			})
		})

		Context("with existing PVC", func() {
			var (
				dataVolume *cdiv1beta1.DataVolume
			)

			BeforeEach(func() {
				dataVolume = &cdiv1beta1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:        dataSourceName,
						Namespace:   internal.GoldenImagesNamespace,
						Annotations: commonAnnotations,
					},
					Spec: cdiv1beta1.DataVolumeSpec{
						Source: &cdiv1beta1.DataVolumeSource{
							Registry: &cdiv1beta1.DataVolumeSourceRegistry{
								URL:        &registryURL,
								PullMethod: &pullMethod,
							},
						},
						Storage: &cdiv1beta1.StorageSpec{
							Resources: core.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse("128Mi"),
								},
							},
						},
					},
				}
				Expect(apiClient.Create(ctx, dataVolume)).To(Succeed())

				Eventually(func() cdiv1beta1.DataVolumePhase {
					foundDv := &cdiv1beta1.DataVolume{}
					Expect(apiClient.Get(ctx, client.ObjectKeyFromObject(dataVolume), foundDv)).To(Succeed())
					return foundDv.Status.Phase
				}, timeout, time.Second).Should(Equal(cdiv1beta1.Succeeded), "DataVolume should successfully import.")

				Eventually(func() bool {
					foundDs := &cdiv1beta1.DataSource{}
					Expect(apiClient.Get(ctx, dataSource.GetKey(), foundDs)).To(Succeed())

					readyCond := getDataSourceReadyCondition(foundDs)
					return readyCond != nil && readyCond.Status == core.ConditionTrue
				}, shortTimeout, time.Second).Should(BeTrue(), "DataSource should have Ready condition true")

				updateSsp(func(foundSsp *ssp.SSP) {
					foundSsp.Spec.CommonTemplates.DataImportCronTemplates = append(foundSsp.Spec.CommonTemplates.DataImportCronTemplates,
						cronTemplate,
					)
				})

				waitUntilDeployed()
			})

			AfterEach(func() {
				err := apiClient.Delete(ctx, dataVolume)
				if !errors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred(), "Failed to delete data volume")
				}
				waitForDeletion(client.ObjectKeyFromObject(dataVolume), &cdiv1beta1.DataVolume{})
			})

			It("[test_id:8110] should not create DataImportCron", func() {
				err := apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())
				Expect(err).To(HaveOccurred())
				Expect(errors.ReasonForError(err)).To(Equal(metav1.StatusReasonNotFound))
			})

			It("[test_id:8114] should not create DataImportCron if DataSource is deleted", func() {
				ds := dataSource.NewResource()
				ds.SetName(dataSource.Name)
				ds.SetNamespace(dataSource.Namespace)

				Expect(apiClient.Delete(ctx, ds)).To(Succeed())

				// Wait until DataSource is recreated.
				Eventually(func() error {
					return apiClient.Get(ctx, dataSource.GetKey(), dataSource.NewResource())
				}, shortTimeout, time.Second).Should(Succeed())

				err := apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())
				Expect(err).To(HaveOccurred())
				Expect(errors.ReasonForError(err)).To(Equal(metav1.StatusReasonNotFound))
			})

			It("[test_id:8113] should create DataImportCron if PVC is deleted", func() {
				err := apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())
				Expect(err).To(HaveOccurred())
				Expect(errors.ReasonForError(err)).To(Equal(metav1.StatusReasonNotFound), "DataImportCron should not exist.")

				Expect(apiClient.Delete(ctx, dataVolume)).To(Succeed())
				waitForDeletion(client.ObjectKeyFromObject(dataVolume), &cdiv1beta1.DataVolume{})

				Eventually(func() error {
					return apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())
				}, shortTimeout, time.Second).Should(Succeed())
			})

			Context("with CDI label on DataSource", func() {
				BeforeEach(func() {
					Eventually(func() error {
						ds := &cdiv1beta1.DataSource{}
						Expect(apiClient.Get(ctx, dataSource.GetKey(), ds))

						if ds.GetLabels() == nil {
							ds.SetLabels(map[string]string{})
						}
						// Removing cleanup label, otherwise the DS would be removed by CDI and the following tests would fail.
						// This is to remove side effects between tests. When the DIC is removed in one of the AfterEach() blocks,
						// CDI adds this label to the DS.
						delete(ds.GetLabels(), cdiCleanupLabel)

						ds.GetLabels()[cdiLabel] = "test-value"

						return apiClient.Update(ctx, ds)
					}, shortTimeout, time.Second).Should(Succeed())
				})

				AfterEach(func() {
					Eventually(func() error {
						ds := &cdiv1beta1.DataSource{}
						Expect(apiClient.Get(ctx, dataSource.GetKey(), ds))
						for key := range ds.GetLabels() {
							if strings.HasPrefix(key, cdiLabelPrefix) {
								delete(ds.GetLabels(), key)
							}
						}
						return apiClient.Update(ctx, ds)
					}, shortTimeout, time.Second).Should(Succeed())
				})

				It("[test_id:8116] should create DataImportCron", func() {
					Eventually(func() error {
						return apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())
					}, shortTimeout, time.Second).Should(Succeed())
				})

				It("[test_id:8297] should delete DataImportCron, when CDI label is removed from DataSource", func() {
					Eventually(func() error {
						return apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())
					}, shortTimeout, time.Second).Should(Succeed())

					waitUntilDeployed()

					Eventually(func() error {
						ds := &cdiv1beta1.DataSource{}
						Expect(apiClient.Get(ctx, dataSource.GetKey(), ds))
						delete(ds.GetLabels(), cdiLabel)
						return apiClient.Update(ctx, ds)
					}, shortTimeout, time.Second).Should(Succeed())

					Eventually(func() metav1.StatusReason {
						err := apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())
						return errors.ReasonForError(err)
					}, timeout, time.Second).Should(Equal(metav1.StatusReasonNotFound), "DataImportCron should not exist.")
				})

				It("[test_id:8298] should restore DataSource, when CDI label is removed", func() {
					// Wait until DataImportCron imports PVC and changes data source
					Eventually(func() (bool, error) {
						cron := &cdiv1beta1.DataImportCron{}
						err := apiClient.Get(ctx, dataImportCron.GetKey(), cron)
						if err != nil {
							return false, err
						}
						return cron.Status.LastImportTimestamp.IsZero(), nil
					}, timeout, time.Second).Should(BeFalse(), "DataImportCron did not finish importing.")

					// Get DataSource with spec pointing to new PVC
					autoUpdateDataSource := &cdiv1beta1.DataSource{}
					Expect(apiClient.Get(ctx, dataSource.GetKey(), autoUpdateDataSource)).To(Succeed())

					// Remove label
					Eventually(func() error {
						ds := &cdiv1beta1.DataSource{}
						Expect(apiClient.Get(ctx, dataSource.GetKey(), ds))
						delete(ds.GetLabels(), cdiLabel)
						return apiClient.Update(ctx, ds)
					}, shortTimeout, time.Second).Should(Succeed())

					// Wait until DataSource is reverted
					Eventually(func() client.Object {
						ds := &cdiv1beta1.DataSource{}
						Expect(apiClient.Get(ctx, dataSource.GetKey(), ds)).To(Succeed())
						return ds
					}, shortTimeout, time.Second).ShouldNot(EqualResource(&dataSource, autoUpdateDataSource))
				})
			})

			Context("with custom NameSpace", func() {
				var (
					customNamespace core.Namespace
				)

				BeforeEach(func() {
					customNamespace = core.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "custom-namespace",
						},
					}
					Expect(apiClient.Create(ctx, &customNamespace)).To(Succeed())

					cronTemplate.Namespace = customNamespace.Name
					dataImportCron.Namespace = customNamespace.Name
					updateSsp(func(foundSsp *ssp.SSP) {
						foundSsp.Spec.CommonTemplates.DataImportCronTemplates = append(foundSsp.Spec.CommonTemplates.DataImportCronTemplates,
							cronTemplate,
						)
					})

					waitUntilDeployed()
				})

				AfterEach(func() {
					Expect(apiClient.Delete(ctx, &customNamespace)).To(Succeed())
					waitForDeletion(client.ObjectKeyFromObject(&customNamespace), &core.Namespace{})
				})

				It("[test_id:????] should create DataImportCron", func() {
					Eventually(func() error {
						return apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())
					}, shortTimeout, time.Second).Should(Succeed())
				})
			})
		})

		Context("with DataImportCron cleanup", func() {
			var cron *cdiv1beta1.DataImportCron

			AfterEach(func() {
				if cron != nil {
					err := apiClient.Delete(ctx, cron)
					if !errors.IsNotFound(err) {
						Expect(err).ToNot(HaveOccurred(), "Failed to delete DataImportCron")
					}
					cron = nil
				}
			})

			It("[test_id:7453] should keep DataImportCron if not owned by operator", func() {
				retentionPolicyNone := cdiv1beta1.DataImportCronRetainNone
				cron = &cdiv1beta1.DataImportCron{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-not-in-ssp",
						Namespace:    internal.GoldenImagesNamespace,
						Annotations:  commonAnnotations,
					},
					Spec: cdiv1beta1.DataImportCronSpec{
						Schedule:          cronSchedule,
						ManagedDataSource: "test-not-in-ssp",
						RetentionPolicy:   &retentionPolicyNone,
						Template: cdiv1beta1.DataVolume{
							Spec: cdiv1beta1.DataVolumeSpec{
								Source: &cdiv1beta1.DataVolumeSource{
									Registry: &cdiv1beta1.DataVolumeSourceRegistry{
										URL:        &registryURL,
										PullMethod: &pullMethod,
									},
								},
								Storage: &cdiv1beta1.StorageSpec{
									Resources: core.ResourceRequirements{
										Requests: core.ResourceList{
											core.ResourceStorage: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				}

				Expect(apiClient.Create(ctx, cron)).To(Succeed())

				// Trigger reconciliation by adding an annotation to SSP.
				updateSsp(func(foundSsp *ssp.SSP) {
					if foundSsp.Annotations == nil {
						foundSsp.Annotations = map[string]string{}
					}
					foundSsp.Annotations["ssp-trigger-reconcile"] = "true"
				})
				waitUntilDeployed()

				err := apiClient.Get(ctx, client.ObjectKeyFromObject(cron), &cdiv1beta1.DataImportCron{})
				Expect(err).ToNot(HaveOccurred(), "unrelated DataImportCron was removed")
			})
		})
	})
})

func getDataSourceReadyCondition(dataSource *cdiv1beta1.DataSource) *cdiv1beta1.DataSourceCondition {
	for i := range dataSource.Status.Conditions {
		condition := &dataSource.Status.Conditions[i]
		if condition.Type == cdiv1beta1.DataSourceReady {
			return condition
		}
	}
	return nil
}
