package util

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// ForceRunModeEnv indicates if the operator should be forced to run in either local
// or cluster mode (currently only used for local mode)
var ForceRunModeEnv = "OSDK_FORCE_RUN_MODE"

type RunModeType string

const (
	LocalRunMode   RunModeType = "local"
	ClusterRunMode RunModeType = "cluster"

	// PodNameEnvVar is the constant for env variable POD_NAME
	// which is the name of the current pod.
	PodNameEnvVar = "POD_NAME"
)

// ErrNoNamespace indicates that a namespace could not be found for the current
// environment
var ErrNoNamespace = fmt.Errorf("namespace not found for current environment")

// ErrRunLocal indicates that the operator is set to run in local mode (this error
// is returned by functions that only work on operators running in cluster mode)
var ErrRunLocal = fmt.Errorf("operator run mode forced to local")

func GetOperatorNamespaceFromEnv() string {
	return os.Getenv(OperatorNamespaceEnv)
}

func IsRunModeLocal() bool {
	return os.Getenv(ForceRunModeEnv) == string(LocalRunMode)
}

// GetOperatorNamespace returns the namespace the operator should be running in.
var GetOperatorNamespace = func(logger logr.Logger) (string, error) {
	if IsRunModeLocal() {
		return "", ErrRunLocal
	}
	nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoNamespace
		}
		return "", err
	}
	ns := strings.TrimSpace(string(nsBytes))
	logger.Info("Found namespace", "Namespace", ns)
	return ns, nil
}

// ToUnstructured converts an arbitrary object (which MUST obey the
// k8s object conventions) to an Unstructured
func ToUnstructured(obj runtime.Object, c client.Client) (*unstructured.Unstructured, error) {
	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if apiVersion == "" || kind == "" {
		gvk, err := apiutil.GVKForObject(obj, c.Scheme())
		if err != nil {
			return nil, err
		}
		ta, err := meta.TypeAccessor(obj)
		if err != nil {
			return nil, err
		}
		ta.SetKind(gvk.Kind)
		ta.SetAPIVersion(gvk.GroupVersion().String())
	}

	u := &unstructured.Unstructured{}
	var err error
	u.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(obj)

	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetRuntimeObject will query the apiserver for the object
func GetRuntimeObject(ctx context.Context, c client.Client, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	return c.Get(ctx, key, obj)
}

// ComponentResourceRemoval removes the resource `obj` if it exists and belongs to the HCO
// with wait=true it will wait, (util ctx timeout, please set it!) for the resource to be effectively deleted
func ComponentResourceRemoval(ctx context.Context, c client.Client, obj client.Object, hcoName string, logger logr.Logger, dryRun bool, wait bool, protectNonHCOObjects bool) (bool, error) {

	logger.Info("Removing resource", "name", obj.GetName(), "namespace", obj.GetNamespace(), "GVK", obj.GetObjectKind().GroupVersionKind(), "dryRun", dryRun)

	resource, err := ToUnstructured(obj, c)
	if err != nil {
		logger.Error(err, "Failed to convert object to Unstructured")
		return false, err
	}

	ok, err := getResourceForDeletion(ctx, c, resource, logger)
	if !ok {
		return false, err
	}

	if protectNonHCOObjects && !shouldDeleteResource(resource, hcoName, logger) {
		return false, nil
	}

	opts := getDeletionOption(dryRun, wait)

	if deleted, err := doDeleteResource(ctx, c, resource, opts); !deleted {
		return deleted, err
	}

	if !wait || dryRun { // no need to wait
		return !dryRun, nil
	}

	return validateDeletion(ctx, c, resource)
}

func doDeleteResource(ctx context.Context, c client.Client, resource *unstructured.Unstructured, opts *client.DeleteOptions) (bool, error) {
	err := c.Delete(ctx, resource, opts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// to be idempotent if called on a object that was
			// already marked for deletion in a previous reconciliation loop
			return false, nil
		}
		// failure
		return false, err
	}
	return true, err
}

func shouldDeleteResource(resource *unstructured.Unstructured, hcoName string, logger logr.Logger) bool {
	labels := resource.GetLabels()
	if app, labelExists := labels[AppLabel]; !labelExists || app != hcoName {
		logger.Info("Existing resource wasn't deployed by HCO, ignoring", "Kind", resource.GetObjectKind())
		return false
	}
	return true
}

func getDeletionOption(dryRun bool, wait bool) *client.DeleteOptions {
	opts := &client.DeleteOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	if wait {
		opts.PropagationPolicy = ptr.To(metav1.DeletePropagationForeground)
	}
	return opts
}

func getResourceForDeletion(ctx context.Context, c client.Client, resource *unstructured.Unstructured, logger logr.Logger) (bool, error) {
	err := c.Get(ctx, types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}, resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Resource doesn't exist, there is nothing to remove", "Kind", resource.GetObjectKind())
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func validateDeletion(ctx context.Context, c client.Client, resource *unstructured.Unstructured) (bool, error) {
	for {
		err := c.Get(ctx, types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}, resource)
		if apierrors.IsNotFound(err) {
			// success!
			return true, nil
		}
		select {
		case <-ctx.Done():
			// failed to delete in time
			return false, fmt.Errorf("timed out waiting for %q - %q to be deleted", resource.GetObjectKind(), resource.GetName())
		case <-time.After(100 * time.Millisecond):
			// do nothing, try again
		}
	}
}

// EnsureDeleted calls ComponentResourceRemoval if the runtime object exists
// with wait=true it will wait, (util ctx timeout, please set it!) for the resource to be effectively deleted
func EnsureDeleted(ctx context.Context, c client.Client, obj client.Object, hcoName string, logger logr.Logger, dryRun bool, wait bool, protectNonHCOObjects bool) (bool, error) {
	err := GetRuntimeObject(ctx, c, obj)

	if err != nil {
		var gdferr *discovery.ErrGroupDiscoveryFailed
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) || errors.As(err, &gdferr) {
			logger.Info("Resource doesn't exist, there is nothing to remove", "Kind", obj.GetObjectKind())
			return false, nil
		}

		logger.Error(err, "failed to get object from kubernetes", "Kind", obj.GetObjectKind())
		return false, err
	}

	return ComponentResourceRemoval(ctx, c, obj, hcoName, logger, dryRun, wait, protectNonHCOObjects)
}

var hcoKvIoVersion string

func GetHcoKvIoVersion() string {
	if hcoKvIoVersion == "" {
		hcoKvIoVersion = os.Getenv(HcoKvIoVersionName)
	}
	return hcoKvIoVersion
}

func AddCrToTheRelatedObjectList(relatedObjects *[]corev1.ObjectReference, found client.Object, scheme *runtime.Scheme) (bool, error) {
	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(scheme, found)
	if err != nil {
		return false, err
	}

	existingRef, err := objectreferencesv1.FindObjectReference(*relatedObjects, *objectRef)
	if err != nil {
		return false, err
	}
	if existingRef == nil || !reflect.DeepEqual(existingRef, objectRef) {
		err = objectreferencesv1.SetObjectReference(relatedObjects, *objectRef)
		if err != nil {
			return false, err
		}
		// Eventually remove outdated reference with a different APIVersion
		for _, ref := range *relatedObjects {
			if ref.Kind == objectRef.Kind && ref.Namespace == objectRef.Namespace && ref.Name == objectRef.Name && ref.APIVersion != objectRef.APIVersion {
				err = objectreferencesv1.RemoveObjectReference(relatedObjects, ref)
				if err != nil {
					return false, err
				}
			}
		}
		return true, nil
	}

	return false, nil
}

func GetLabels(hcName string, component AppComponent) map[string]string {
	return map[string]string{
		AppLabel:          hcName,
		AppLabelManagedBy: OperatorName,
		AppLabelVersion:   GetHcoKvIoVersion(),
		AppLabelPartOf:    HyperConvergedCluster,
		AppLabelComponent: string(component),
	}
}
