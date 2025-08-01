package wasp_agent

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

func NewServiceAccountHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewServiceAccountHandler(Client, Scheme, newSA),
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newSA(hc)
		},
	)
}

func newSA(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      waspAgentServiceAccountName,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, AppComponentWaspAgent),
		},
	}
}
