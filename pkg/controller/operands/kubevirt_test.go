package operands

import (
	"context"
	"fmt"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/commonTestUtils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/openshift/custom-resource-status/testlib"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"
)

var _ = Describe("KubeVirt Operand", func() {

	const (
		// fake KV managed FGs
		fgEnabled  = "fgEnabled"
		fgMissing  = "fgMissing"
		fgDisabled = "fgDisabled"
		fgNoChange = "fgNoChange"
	)

	Context("KubeVirt Priority Classes", func() {

		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		BeforeEach(func() {
			hco = commonTestUtils.NewHco()
			req = commonTestUtils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := NewKubeVirtPriorityClass(hco)
			cl := commonTestUtils.InitClient([]runtime.Object{})
			handler := (*genericOperand)(newKvPriorityClassHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			key, err := client.ObjectKeyFromObject(expectedResource)
			Expect(err).ToNot(HaveOccurred())
			foundResource := &schedulingv1.PriorityClass{}
			Expect(cl.Get(context.TODO(), key, foundResource)).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Value).To(Equal(expectedResource.Value))
			Expect(foundResource.GlobalDefault).To(Equal(expectedResource.GlobalDefault))
		})

		It("should do nothing if already exists", func() {
			expectedResource := NewKubeVirtPriorityClass(hco)
			cl := commonTestUtils.InitClient([]runtime.Object{expectedResource})
			handler := (*genericOperand)(newKvPriorityClassHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		DescribeTable("should update if something changed", func(modifiedResource *schedulingv1.PriorityClass) {
			cl := commonTestUtils.InitClient([]runtime.Object{modifiedResource})
			handler := (*genericOperand)(newKvPriorityClassHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			expectedResource := NewKubeVirtPriorityClass(hco)
			key, err := client.ObjectKeyFromObject(expectedResource)
			Expect(err).ToNot(HaveOccurred())
			foundResource := &schedulingv1.PriorityClass{}
			Expect(cl.Get(context.TODO(), key, foundResource))
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Value).To(Equal(expectedResource.Value))
			Expect(foundResource.GlobalDefault).To(Equal(expectedResource.GlobalDefault))
		},
			Entry("with modified value",
				&schedulingv1.PriorityClass{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "scheduling.k8s.io/v1",
						Kind:       "PriorityClass",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "kubevirt-cluster-critical",
					},
					Value:         1,
					GlobalDefault: false,
					Description:   "",
				}),
			Entry("with modified global default",
				&schedulingv1.PriorityClass{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "scheduling.k8s.io/v1",
						Kind:       "PriorityClass",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "kubevirt-cluster-critical",
					},
					Value:         1000000000,
					GlobalDefault: true,
					Description:   "",
				}),
		)

	})

	Context("KubeVirt Config", func() {

		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		updatableKeys := [...]string{virtconfig.SmbiosConfigKey, virtconfig.MachineTypeKey, virtconfig.SELinuxLauncherTypeKey, virtconfig.FeatureGatesKey}
		removeKeys := [...]string{virtconfig.MigrationsConfigKey}
		unupdatableKeys := [...]string{virtconfig.NetworkInterfaceKey}

		BeforeEach(func() {
			hco = commonTestUtils.NewHco()
			req = commonTestUtils.NewReq(hco)

			os.Setenv("SMBIOS", "new-smbios-value-that-we-have-to-set")
			os.Setenv("MACHINETYPE", "new-machinetype-value-that-we-have-to-set")
		})

		It("should create if not present", func() {
			expectedResource := NewKubeVirtConfigForCR(req.Instance, commonTestUtils.Namespace)
			cl := commonTestUtils.InitClient([]runtime.Object{})

			handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)

			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commonTestUtils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := commonTestUtils.InitClient([]runtime.Object{hco, expectedResource})
			handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should update only a few keys and only when in upgrade mode", func() {
			expectedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data[virtconfig.SmbiosConfigKey] = "old-smbios-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.MachineTypeKey] = "old-machinetype-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.SELinuxLauncherTypeKey] = "old-selinuxlauncher-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.FeatureGatesKey] = "old-featuregates-value-that-we-have-to-update"
			// value that we should remove if configured
			outdatedResource.Data[virtconfig.MigrationsConfigKey] = "old-migrationsconfig-value-that-we-should-remove"
			// values we should preserve
			outdatedResource.Data[virtconfig.NetworkInterfaceKey] = "old-defaultnetworkinterface-value-that-we-should-preserve"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, outdatedResource})
			handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))

			// force upgrade mode
			req.UpgradeMode = true
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			for _, k := range updatableKeys {
				Expect(foundResource.Data[k]).To(Not(Equal(outdatedResource.Data[k])))
				Expect(foundResource.Data[k]).To(Equal(expectedResource.Data[k]))
			}
			for _, k := range unupdatableKeys {
				Expect(foundResource.Data[k]).To(Equal(outdatedResource.Data[k]))
				Expect(foundResource.Data[k]).To(Not(Equal(expectedResource.Data[k])))
			}
			for _, k := range removeKeys {
				Expect(outdatedResource.Data).To(HaveKey(k))
				Expect(expectedResource.Data).To(Not(HaveKey(k)))
				Expect(foundResource.Data).To(Not(HaveKey(k)))
			}
		})

		It("should not touch it when not in in upgrade mode", func() {
			expectedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data[virtconfig.SmbiosConfigKey] = "old-smbios-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.MachineTypeKey] = "old-machinetype-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.SELinuxLauncherTypeKey] = "old-selinuxlauncher-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.FeatureGatesKey] = "old-featuregates-value-that-we-have-to-update"
			// values we should preserve
			outdatedResource.Data[virtconfig.MigrationsConfigKey] = "old-migrationsconfig-value-that-we-should-preserve"
			outdatedResource.Data[virtconfig.DefaultNetworkInterface] = "old-defaultnetworkinterface-value-that-we-should-preserve"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, outdatedResource})
			handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))

			// ensure that we are not in upgrade mode
			req.UpgradeMode = false

			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.Data).To(Equal(outdatedResource.Data))
			Expect(foundResource.Data).To(Not(Equal(expectedResource.Data)))
		})

		Context("Feature Gates", func() {
			origKvFeatureGates := managedKvFeatureGates

			const (
				userModifiedFgs = "userModifiedFg1,userModifiedFg2,userModifiedFg3"
			)
			BeforeEach(func() {
				// list of fake KV managed FGs
				managedKvFeatureGates = []string{fgEnabled, fgMissing, fgDisabled, fgNoChange}
			})

			AfterEach(func() {
				managedKvFeatureGates = origKvFeatureGates
			})

			It("should have a list of enabled features that are managed by the HCO CR", func() {
				existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
				By("KV CR should contain the fgEnabled feature gate", func() {
					Expect(existingResource.Data[virtconfig.FeatureGatesKey]).Should(Equal(cmFeatureGates))
				})
			})

			It("should add the feature gates if they exist and enabled in HyperConverged CR", func() {
				hco.Spec.FeatureGates = map[string]bool{
					fgEnabled:  true,
					fgDisabled: false,
				}

				existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
				By("KV CR should contain the fgEnabled feature gate", func() {
					Expect(existingResource.Data[virtconfig.FeatureGatesKey]).Should(Equal(cmFeatureGates + "," + fgEnabled))
				})
			})

			It("should add feature gates if they are set to true", func() {
				existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
				By("Make sure the enabled FG is not there", func() {
					Expect(existingResource.Data[virtconfig.FeatureGatesKey]).Should(Equal(cmFeatureGates))
				})

				hco.Spec.FeatureGates = map[string]bool{
					fgEnabled:  true,
					fgDisabled: false,
				}

				foundResource := &corev1.ConfigMap{}
				reconcileCm(hco, req, true, existingResource, foundResource)

				By("KV CR should contain the enabled feature gate", func() {
					Expect(foundResource.Data[virtconfig.FeatureGatesKey]).Should(Equal(cmFeatureGates + "," + fgEnabled))
				})
			})

			It("should handle feature gates on update", func() {
				existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
				existingResource.Data[virtconfig.FeatureGatesKey] = fmt.Sprintf("%s,%s,%s,%s", cmFeatureGates, fgMissing, fgDisabled, fgNoChange)

				hco.Spec.FeatureGates = map[string]bool{
					fgEnabled:  true,
					fgDisabled: false,
					fgNoChange: true,
				}

				foundResource := &corev1.ConfigMap{}
				reconcileCm(hco, req, true, existingResource, foundResource)

				By("Should add enabled FGs, remove missing FGs, remove disabled FGs and not change existing enabled FGs", func() {
					found := foundResource.Data[virtconfig.FeatureGatesKey]
					Expect(strings.Contains(found, fgEnabled)).To(BeTrue())
					Expect(strings.Contains(found, fgMissing)).To(BeFalse())
					Expect(strings.Contains(found, fgDisabled)).To(BeFalse())
					Expect(strings.Contains(found, fgNoChange)).To(BeTrue())
				})
			})

			It("should remove all KV feature gates if there are no managed KV feature gates in HC", func() {
				existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
				existingResource.Data[virtconfig.FeatureGatesKey] = fmt.Sprintf("%s,%s,%s,%s", cmFeatureGates, fgMissing, fgDisabled, fgNoChange)

				foundResource := &corev1.ConfigMap{}
				reconcileCm(hco, req, true, existingResource, foundResource)

				Expect(foundResource.Data[virtconfig.FeatureGatesKey]).Should(Equal(cmFeatureGates))
			})

			It("should not modify user modified feature gates on update", func() {
				existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
				existingResource.Data[virtconfig.FeatureGatesKey] = fmt.Sprintf("%s,%s,%s,%s", userModifiedFgs, fgMissing, fgDisabled, fgNoChange)

				hco.Spec.FeatureGates = map[string]bool{
					fgEnabled:  true,
					fgDisabled: false,
					fgNoChange: true,
				}

				foundResource := &corev1.ConfigMap{}
				reconcileCm(hco, req, true, existingResource, foundResource)

				By("Should add enabled FGs, remove missing FGs, remove disabled FGs and not change existing enabled FGs", func() {
					found := foundResource.Data[virtconfig.FeatureGatesKey]
					Expect(strings.Contains(found, cmFeatureGates)).To(BeFalse())
					Expect(strings.Contains(found, userModifiedFgs)).To(BeTrue())
					Expect(strings.Contains(found, fgEnabled)).To(BeTrue())
					Expect(strings.Contains(found, fgMissing)).To(BeFalse())
					Expect(strings.Contains(found, fgDisabled)).To(BeFalse())
					Expect(strings.Contains(found, fgNoChange)).To(BeTrue())
				})
			})

			It("should modify user modified feature gates on upgrade", func() {
				existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
				existingResource.Data[virtconfig.FeatureGatesKey] = fmt.Sprintf("%s,%s,%s,%s", userModifiedFgs, fgMissing, fgDisabled, fgNoChange)

				hco.Spec.FeatureGates = map[string]bool{
					fgEnabled:  true,
					fgDisabled: false,
					fgNoChange: true,
				}

				req.UpgradeMode = true
				foundResource := &corev1.ConfigMap{}
				reconcileCm(hco, req, true, existingResource, foundResource)

				By("Should add enabled FGs, remove missing FGs, remove disabled FGs and not change existing enabled FGs", func() {
					found := foundResource.Data[virtconfig.FeatureGatesKey]
					Expect(strings.Contains(found, cmFeatureGates)).To(BeTrue())
					Expect(strings.Contains(found, userModifiedFgs)).To(BeFalse())
					Expect(strings.Contains(found, fgEnabled)).To(BeTrue())
					Expect(strings.Contains(found, fgMissing)).To(BeFalse())
					Expect(strings.Contains(found, fgDisabled)).To(BeFalse())
					Expect(strings.Contains(found, fgNoChange)).To(BeTrue())
				})
			})
		})
	})

	Context("KubeVirt", func() {
		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		BeforeEach(func() {
			hco = commonTestUtils.NewHco()
			req = commonTestUtils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			cl := commonTestUtils.InitClient([]runtime.Object{})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commonTestUtils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := commonTestUtils.InitClient([]runtime.Object{hco, expectedResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.Conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
			Expect(req.Conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
			Expect(req.Conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
		})

		It("should set default UninstallStrategy if missing", func() {
			expectedResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			missingUSResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			missingUSResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", missingUSResource.Namespace, missingUSResource.Name)
			missingUSResource.Spec.UninstallStrategy = ""

			cl := commonTestUtils.InitClient([]runtime.Object{hco, missingUSResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.UninstallStrategy).To(Equal(expectedResource.Spec.UninstallStrategy))
		})

		It("should add node placement if missing in KubeVirt", func() {
			existingResource, err := NewKubeVirt(hco)
			Expect(err).ToNot(HaveOccurred())

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra).To(BeNil())
			Expect(existingResource.Spec.Workloads).To(BeNil())

			Expect(foundResource.Spec.Infra).ToNot(BeNil())
			Expect(foundResource.Spec.Infra.NodePlacement).ToNot(BeNil())
			Expect(foundResource.Spec.Infra.NodePlacement.Affinity).ToNot(BeNil())
			Expect(foundResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(foundResource.Spec.Infra.NodePlacement.NodeSelector["key2"]).Should(Equal("value2"))

			Expect(foundResource.Spec.Workloads).ToNot(BeNil())
			Expect(foundResource.Spec.Workloads.NodePlacement).ToNot(BeNil())
			Expect(foundResource.Spec.Workloads.NodePlacement.Tolerations).Should(Equal(hco.Spec.Workloads.NodePlacement.Tolerations))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should remove node placement if missing in HCO CR", func() {

			hcoNodePlacement := commonTestUtils.NewHco()
			hcoNodePlacement.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			hcoNodePlacement.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			existingResource, err := NewKubeVirt(hcoNodePlacement)
			Expect(err).ToNot(HaveOccurred())

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra).ToNot(BeNil())
			Expect(existingResource.Spec.Workloads).ToNot(BeNil())

			Expect(foundResource.Spec.Infra).To(BeNil())
			Expect(foundResource.Spec.Workloads).To(BeNil())

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should modify node placement according to HCO CR", func() {
			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			existingResource, err := NewKubeVirt(hco)
			Expect(err).ToNot(HaveOccurred())

			// now, modify HCO's node placement
			seconds3 := int64(3)
			hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra).ToNot(BeNil())
			Expect(existingResource.Spec.Infra.NodePlacement).ToNot(BeNil())
			Expect(existingResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(2))
			Expect(existingResource.Spec.Workloads).ToNot(BeNil())

			Expect(existingResource.Spec.Workloads.NodePlacement).ToNot(BeNil())
			Expect(existingResource.Spec.Workloads.NodePlacement.NodeSelector["key1"]).Should(Equal("value1"))

			Expect(foundResource.Spec.Infra).ToNot(BeNil())
			Expect(foundResource.Spec.Infra.NodePlacement).ToNot(BeNil())
			Expect(foundResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(3))

			Expect(foundResource.Spec.Workloads).ToNot(BeNil())
			Expect(foundResource.Spec.Workloads.NodePlacement).ToNot(BeNil())
			Expect(foundResource.Spec.Workloads.NodePlacement.NodeSelector["key1"]).Should(Equal("something else"))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should overwrite node placement if directly set on KV CR", func() {
			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			existingResource, err := NewKubeVirt(hco)
			Expect(err).ToNot(HaveOccurred())

			// mock a reconciliation triggered by a change in KV CR
			req.HCOTriggered = false

			// now, modify KV's node placement
			seconds3 := int64(3)
			existingResource.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})
			existingResource.Spec.Workloads.NodePlacement.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			existingResource.Spec.Infra.NodePlacement.NodeSelector["key1"] = "BADvalue1"
			existingResource.Spec.Workloads.NodePlacement.NodeSelector["key2"] = "BADvalue2"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(3))
			Expect(existingResource.Spec.Workloads.NodePlacement.Tolerations).To(HaveLen(3))
			Expect(existingResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).Should(Equal("BADvalue1"))
			Expect(existingResource.Spec.Workloads.NodePlacement.NodeSelector["key2"]).Should(Equal("BADvalue2"))

			Expect(foundResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(2))
			Expect(foundResource.Spec.Workloads.NodePlacement.Tolerations).To(HaveLen(2))
			Expect(foundResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(foundResource.Spec.Workloads.NodePlacement.NodeSelector["key2"]).Should(Equal("value2"))

			Expect(req.Conditions).To(BeEmpty())
		})

		Context("Feature Gates", func() {
			origKvFeatureGates := managedKvFeatureGates

			BeforeEach(func() {
				// list of fake KV managed FGs
				managedKvFeatureGates = []string{fgEnabled, fgMissing, fgDisabled, fgNoChange}
			})

			AfterEach(func() {
				managedKvFeatureGates = origKvFeatureGates
			})

			It("should add the feature gates if they are set in HyperConverged CR", func() {
				hco.Spec.FeatureGates = map[string]bool{
					fgEnabled:  true,
					fgDisabled: false,
				}

				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())
				By("KV CR should contain the fgEnabled feature gate", func() {
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(fgEnabled))
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(fgMissing))
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(fgDisabled))
				})
			})

			It("should not add the feature gates if FeatureGates map is nil", func() {
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(existingResource.Spec.Configuration.DeveloperConfiguration).To(BeNil())
			})

			It("should add feature gates if they are set to true", func() {
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Spec.FeatureGates = map[string]bool{
					fgEnabled:  true,
					fgDisabled: false,
				}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				By("KV CR should contain the HC enabled managed feature gates", func() {
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(fgEnabled))
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(fgMissing))
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(fgDisabled))
				})
			})

			It("should handle existing feature gates on update", func() {
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())
				existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
					FeatureGates: []string{fgMissing, fgDisabled, fgNoChange},
				}

				hco.Spec.FeatureGates = map[string]bool{
					fgEnabled:  true,
					fgDisabled: false,
					fgNoChange: true,
				}

				By("Make sure the existing KV is with the the expected FGs", func() {
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(fgEnabled))
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(fgMissing))
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(fgDisabled))
					Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(fgNoChange))
				})

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(foundResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())

				By("Should add enabled FGs", func() {
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(fgEnabled))
				})
				By("Should remove missing FGs", func() {
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(fgMissing))
				})
				By("Should remove disabled FGs", func() {
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(fgDisabled))
				})
				By("Should not change existing enabled FGs", func() {
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(fgNoChange))
				})
			})

			It("should remove all KV feature gates if there are no managed KV feature gates in HC", func() {
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())
				existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
					FeatureGates: []string{fgMissing, fgDisabled, fgNoChange},
				}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(foundResource.Spec.Configuration.DeveloperConfiguration).To(BeNil())
			})
		})

		It("should handle conditions", func() {
			expectedResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			expectedResource.Status.Conditions = []kubevirtv1.KubeVirtCondition{
				kubevirtv1.KubeVirtCondition{
					Type:    kubevirtv1.KubeVirtConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "Foo",
					Message: "Bar",
				},
				kubevirtv1.KubeVirtCondition{
					Type:    kubevirtv1.KubeVirtConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
				kubevirtv1.KubeVirtCondition{
					Type:    kubevirtv1.KubeVirtConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
			}
			cl := commonTestUtils.InitClient([]runtime.Object{hco, expectedResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.Conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtNotAvailable",
				Message: "KubeVirt is not available: Bar",
			}))
			Expect(req.Conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtProgressing",
				Message: "KubeVirt is progressing: Bar",
			}))
			Expect(req.Conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtProgressing",
				Message: "KubeVirt is progressing: Bar",
			}))
			Expect(req.Conditions[conditionsv1.ConditionDegraded]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionDegraded,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtDegraded",
				Message: "KubeVirt is degraded: Bar",
			}))
		})

		Context("jsonpath Annotation", func() {
			It("Should create KV object with changes from the annotation", func() {

				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration/featureGates/-",
						"value": "fg2"
					}
				]`}

				kv, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(kv).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(2))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg1"))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg2"))
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())

				quantity, err := resource.ParseQuantity("12m")
				Expect(err).ToNot(HaveOccurred())
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())
				Expect(*kv.Spec.Configuration.CPURequest).Should(Equal(quantity))
			})

			It("Should fail to create KV object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/config/featureGates/-",
						"value": "fg1"
					}
				]`}

				_, err := NewKubeVirt(hco)
				Expect(err).To(HaveOccurred())
			})

			It("Ensure func should create KV object with changes from the annotation", func() {
				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration/featureGates/-",
						"value": "fg2"
					}
				]`}

				expectedResource := NewKubeVirtWithNameOnly(hco)
				cl := commonTestUtils.InitClient([]runtime.Object{hco})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Created).To(BeTrue())
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Err).To(BeNil())

				kv := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						kv),
				).ToNot(HaveOccurred())

				Expect(kv).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(2))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg1"))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg2"))
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())

				quantity, err := resource.ParseQuantity("12m")
				Expect(err).ToNot(HaveOccurred())
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())
				Expect(*kv.Spec.Configuration.CPURequest).Should(Equal(quantity))
			})

			It("Ensure func should fail to create KV object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					}
				]`}

				expectedResource := NewKubeVirtWithNameOnly(hco)
				cl := commonTestUtils.InitClient([]runtime.Object{hco})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).To(HaveOccurred())

				kv := &kubevirtv1.KubeVirt{}

				err := cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					kv)

				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})

			It("Ensure func should update KV object with changes from the annotation", func() {
				existsCdi, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration/featureGates/-",
						"value": "fg2"
					}
				]`}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existsCdi})

				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Updated).To(BeTrue())
				Expect(res.UpgradeDone).To(BeFalse())

				kv := &kubevirtv1.KubeVirt{}

				expectedResource := NewKubeVirtWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						kv),
				).ToNot(HaveOccurred())

				Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(2))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg1"))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg2"))
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())

				quantity, err := resource.ParseQuantity("12m")
				Expect(err).ToNot(HaveOccurred())
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())
				Expect(*kv.Spec.Configuration.CPURequest).Should(Equal(quantity))
			})

			It("Ensure func should fail to update KV object with wrong jsonPatch", func() {
				existsKv, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "notExistsOp",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					}
				]`}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existsKv})

				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).To(HaveOccurred())

				kv := &kubevirtv1.KubeVirt{}

				expectedResource := NewKubeVirtWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						kv),
				).ToNot(HaveOccurred())

				Expect(kv.Spec.Configuration.DeveloperConfiguration).To(BeNil())
				Expect(kv.Spec.Configuration.CPURequest).To(BeNil())

			})
		})

		Context("Cache", func() {
			cl := commonTestUtils.InitClient([]runtime.Object{})
			handler := newKubevirtHandler(cl, commonTestUtils.GetScheme())

			It("should start with empty cache", func() {
				Expect(handler.hooks.(*kubevirtHooks).cache).To(BeNil())
			})

			It("should update the cache when reading full CR", func() {
				cr, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cr).ToNot(BeNil())
				Expect(handler.hooks.(*kubevirtHooks).cache).ToNot(BeNil())

				By("compare pointers to make sure cache is working", func() {
					Expect(handler.hooks.(*kubevirtHooks).cache == cr).Should(BeTrue())

					crII, err := handler.hooks.getFullCr(hco)
					Expect(err).ToNot(HaveOccurred())
					Expect(crII).ToNot(BeNil())
					Expect(cr == crII).Should(BeTrue())
				})
			})

			It("should remove the cache on reset", func() {
				handler.hooks.(*kubevirtHooks).reset()
				Expect(handler.hooks.(*kubevirtHooks).cache).To(BeNil())
			})

			It("check that reset actually cause creating of a new cached instance", func() {
				crI, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(crI).ToNot(BeNil())
				Expect(handler.hooks.(*kubevirtHooks).cache).ToNot(BeNil())

				handler.hooks.(*kubevirtHooks).reset()
				Expect(handler.hooks.(*kubevirtHooks).cache).To(BeNil())

				crII, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(crII).ToNot(BeNil())
				Expect(handler.hooks.(*kubevirtHooks).cache).ToNot(BeNil())

				Expect(crI == crII).To(BeFalse())
				Expect(handler.hooks.(*kubevirtHooks).cache == crI).To(BeFalse())
				Expect(handler.hooks.(*kubevirtHooks).cache == crII).To(BeTrue())
			})
		})
	})
})

func reconcileCm(hco *hcov1beta1.HyperConverged, req *common.HcoRequest, expectUpdate bool, existingCM, foundCm *corev1.ConfigMap) {
	cl := commonTestUtils.InitClient([]runtime.Object{hco, existingCM})
	handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))
	res := handler.ensure(req)
	if expectUpdate {
		ExpectWithOffset(1, res.Updated).To(BeTrue())
	} else {
		ExpectWithOffset(1, res.Updated).To(BeFalse())
	}
	ExpectWithOffset(1, res.Err).ToNot(HaveOccurred())

	ExpectWithOffset(1,
		cl.Get(context.TODO(),
			types.NamespacedName{Name: existingCM.Name, Namespace: existingCM.Namespace},
			foundCm),
	).To(BeNil())
}
