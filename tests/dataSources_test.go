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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	data_sources "kubevirt.io/ssp-operator/internal/operands/data-sources"
)

var _ = Describe("DataSources", func() {
	var (
		expectedLabels map[string]string

		viewRole        testResource
		viewRoleBinding testResource
		editClusterRole testResource
		goldenImageNS   testResource
	)

	BeforeEach(func() {
		expectedLabels = expectedLabelsFor("data-sources", common.AppComponentTemplating)
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

			table.DescribeTable("regular service account DV RBAC allowed", expectUserCan,
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

			table.DescribeTable("regular service account DV RBAC denied", expectUserCannot,
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

			table.DescribeTable("regular service account PVC RBAC allowed", expectUserCan,
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

			table.DescribeTable("regular service account PVC RBAC denied", expectUserCannot,
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

			table.DescribeTable("regular service account DataSource RBAC allowed", expectUserCan,
				table.Entry("[test_id:7466] should be able to 'get' datasources",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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
							Namespace: ssp.GoldenImagesNSname,
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

					table.Entry("[test_id:7452]: ServiceAccounts with edit role can create DataSources",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datasources",
							},
						}),
					table.Entry("[test_id:7451]: ServiceAccounts with edit role can delete DataSources",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datasources",
							},
						}),

					table.Entry("[test_id:7449]: ServiceAccounts with edit role can create DataImportCrons",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "dataimportcrons",
							},
						}),
					table.Entry("[test_id:7448]: ServiceAccounts with edit role can delete DataImportCrons",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: ssp.GoldenImagesNSname,
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

	Context("DataImportCron", func() {
		const cronSchedule = "* * * * *"
		var registryURL = "docker://quay.io/kubevirt/cirros-container-disk-demo"

		var (
			cronTemplate ssp.DataImportCronTemplate

			dataImportCron testResource
		)

		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			cronTemplate = ssp.DataImportCronTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-data-import-cron",
				},
				Spec: cdiv1beta1.DataImportCronSpec{
					Schedule:          cronSchedule,
					ManagedDataSource: "test-data-import-cron",
					Template: cdiv1beta1.DataVolume{
						Spec: cdiv1beta1.DataVolumeSpec{
							Source: &cdiv1beta1.DataVolumeSource{
								Registry: &cdiv1beta1.DataVolumeSourceRegistry{
									URL: &registryURL,
								},
							},
							PVC: &core.PersistentVolumeClaimSpec{
								AccessModes: []core.PersistentVolumeAccessMode{
									core.ReadWriteMany,
								},
								Resources: core.ResourceRequirements{
									Requests: core.ResourceList{
										core.ResourceStorage: resource.MustParse("16Mi"),
									},
								},
							},
						},
					},
				},
			}

			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.CommonTemplates.DataImportCronTemplates = append(foundSsp.Spec.CommonTemplates.DataImportCronTemplates,
					cronTemplate,
				)
			})

			waitUntilDeployed()

			dataImportCron = testResource{
				Name:           cronTemplate.Name,
				Namespace:      ssp.GoldenImagesNSname,
				Resource:       &cdiv1beta1.DataImportCron{},
				ExpectedLabels: expectedLabels,
			}
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		It("[test_id:7469] should create DataImportCron", func() {
			Expect(apiClient.Get(ctx, dataImportCron.GetKey(), dataImportCron.NewResource())).To(Succeed(), "custom DataImportCron not created")
		})

		It("[test_id:7467] should set app labels on DataImportCron", func() {
			expectAppLabels(&dataImportCron)
		})

		It("[test_id:7458] should recreate DataImportCron after delete", func() {
			expectRecreateAfterDelete(&dataImportCron)
		})

		It("[test_id:TODO] should update DataImportCron if updated in SSP CR", func() {
			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.CommonTemplates.DataImportCronTemplates[0].
					Spec.Template.Spec.PVC.Resources.Requests[core.ResourceStorage] = resource.MustParse("32Mi")
			})

			waitUntilDeployed()

			cron := &cdiv1beta1.DataImportCron{}
			Expect(apiClient.Get(ctx, dataImportCron.GetKey(), cron)).To(Succeed())
			Expect(cron.Spec.Template.Spec.PVC.Resources.Requests).To(HaveKeyWithValue(core.ResourceStorage, resource.MustParse("32Mi")))
		})

		It("[test_id:7455] should remove DataImportCron if removed from SSP CR", func() {
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
				cron = &cdiv1beta1.DataImportCron{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-not-in-ssp",
						Namespace:    ssp.GoldenImagesNSname,
					},
					Spec: cdiv1beta1.DataImportCronSpec{
						Schedule:          cronSchedule,
						ManagedDataSource: "test-not-in-ssp",
						Template: cdiv1beta1.DataVolume{
							Spec: cdiv1beta1.DataVolumeSpec{
								Source: &cdiv1beta1.DataVolumeSource{
									Registry: &cdiv1beta1.DataVolumeSourceRegistry{
										URL: &registryURL,
									},
								},
								PVC: &core.PersistentVolumeClaimSpec{
									AccessModes: []core.PersistentVolumeAccessMode{
										core.ReadWriteMany,
									},
									Resources: core.ResourceRequirements{
										Requests: core.ResourceList{
											core.ResourceStorage: resource.MustParse("16Mi"),
										},
									},
								},
							},
						},
					},
				}

				Expect(apiClient.Create(ctx, cron)).To(Succeed())

				waitUntilDeployed()

				err := apiClient.Get(ctx, client.ObjectKeyFromObject(cron), &cdiv1beta1.DataImportCron{})
				Expect(err).ToNot(HaveOccurred(), "unrelated DataImportCron was removed")
			})
		})
	})
})
