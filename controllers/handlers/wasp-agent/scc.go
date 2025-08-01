package wasp_agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"

	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

func NewSecurityContextConstraintHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewSecurityContextConstraintsHandler(Client, Scheme, newSCC),
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newSCCWithNameOnly(hc)
		},
	)
}

func newSCC(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   waspAgentSCCName,
			Labels: operands.GetLabels(hc, AppComponentWaspAgent),
		},
		AllowPrivilegedContainer: true,
		AllowHostDirVolumePlugin: true,
		AllowHostIPC:             true,
		AllowHostNetwork:         true,
		AllowHostPID:             true,
		AllowHostPorts:           true,
		ReadOnlyRootFilesystem:   false,
		DefaultAddCapabilities:   nil,
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyRunAsAny,
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyRunAsAny,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyRunAsAny,
		},
		Users: []string{
			fmt.Sprintf("system:serviceaccount:%s:%s", hc.Namespace, waspAgentServiceAccountName),
		},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeAll,
		},
		AllowedCapabilities: []corev1.Capability{
			"*",
		},
		AllowedUnsafeSysctls: []string{
			"*",
		},
		SeccompProfiles: []string{
			"*",
		},
	}
}

func newSCCWithNameOnly(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   waspAgentSCCName,
			Labels: operands.GetLabels(hc, AppComponentWaspAgent),
		},
	}
}
