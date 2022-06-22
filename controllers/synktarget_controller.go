/*
Copyright 2022 VerwaerdeWim.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	synkv1alpha1 "github.com/VerwaerdeWim/Synk/api/v1alpha1"
)

// SynkTargetReconciler reconciles a SynkTarget object
type SynkTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

var (
	controllerLog = ctrl.Log.WithName("synk controller")
	cancel        = make(map[string]context.CancelFunc)
)

//+kubebuilder:rbac:groups=synk.io,resources=synktargets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=synk.io,resources=synktargets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=synk.io,resources=synktargets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SynkTarget object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *SynkTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	controllerLog.Info("Reconcile")

	synkTarget := &synkv1alpha1.SynkTarget{}
	err := r.Get(ctx, req.NamespacedName, synkTarget)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			controllerLog.Info("Synk resource is deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	connection := synkTarget.Spec.Connection

	// remoteResource := synkTarget.Spec.RemoteResource
	// controllerLog.Info("Synk", "Connection", connection.Host, "RemoteResource", remoteResource)

	// if synkTarget.Status.Conditions != nil {
	// 	patch := client.MergeFrom(synkTarget.DeepCopy())
	// 	for i := range synkTarget.Status.Conditions {
	// 		condition := &synkTarget.Status.Conditions[i]
	// 		condition.LastTransitionTime = metav1.NewTime(time.Now())
	// 		condition.Status = metav1.ConditionFalse
	// 		condition.Message = "Synk operator is starting"
	// 		condition.Reason = "Startup"
	// 	}
	// 	r.Status().Patch(ctx, synkTarget, patch)
	// } else {
	// 	connectedCondition := createStartCondition("Connected", "Synk operator is starting", "Startup")

	// 	remoteresourceCondition := createStartCondition("RemoteResourceFound", "Synk operator is starting", "Startup")

	// 	ownresourceCondition := createStartCondition("OwnResourceFound", "Synk operator is starting", "Startup")

	// 	synkTarget.Status.Conditions = append(synkTarget.Status.Conditions, connectedCondition, remoteresourceCondition, ownresourceCondition)

	// 	r.Status().Update(ctx, synkTarget)
	// }

	config, err := createConfig(connection)
	if err != nil {
		controllerLog.Error(err, "Creating config from connection parameters failed")
		return ctrl.Result{}, err
	}

	for index := range synkTarget.Spec.Resources {
		if synkTarget.Spec.Resources[index].Names != nil {
			for _, name := range synkTarget.Spec.Resources[index].Names {
				c, cancelfunc := context.WithCancel(ctx)
				cancel[getKey(config.Host, &synkTarget.Spec.Resources[index], name)] = cancelfunc
				go r.watchResource(c, &synkTarget.Spec.Resources[index], name, config)
			}
		} else {
			c, cancelfunc := context.WithCancel(ctx)
			cancel[getKey(config.Host, &synkTarget.Spec.Resources[index], "")] = cancelfunc
			go r.watchResource(c, &synkTarget.Spec.Resources[index], "", config)
		}
	}

	// if synkTarget.Spec.OwnResource.Namespace == "" {
	// 	synkTarget.Spec.OwnResource.Namespace = synkTarget.Spec.RemoteResource.Namespace
	// }
	// if synkTarget.Spec.OwnResource.Name == "" {
	// 	synkTarget.Spec.OwnResource.Name = synkTarget.Spec.RemoteResource.Name
	// }

	// wg.Add(1)

	return ctrl.Result{}, nil
}

func (r *SynkTargetReconciler) initWatch(ctx context.Context, resource *synkv1alpha1.Resource, name string, config *rest.Config) <-chan watch.Event {
	controllerLog.Info("trying to connect", "Host", config.Host, "Name", name)
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		controllerLog.Error(err, "Creation dynamic client failed.")
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			watcher, err := createWatch(ctx, client, resource, name)
			if err != nil {
				controllerLog.Info("Watch of resource failed. Trying to reconnect in 5 seconds", "Host", config.Host, "Name", name, "err", err)
				time.Sleep(5 * time.Second)
			} else {
				controllerLog.Info("connection established", "Host", config.Host, "Name", name)
				return watcher.ResultChan()
			}
		}
	}
}

func createWatch(ctx context.Context, client dynamic.Interface, resource *synkv1alpha1.Resource, name string) (watch.Interface, error) {
	if name == "" {
		return client.Resource(schema.GroupVersionResource{
			Group:    resource.Group,
			Version:  resource.Version,
			Resource: resource.ResourceType,
		}).Namespace(resource.Namespace).Watch(ctx, metav1.ListOptions{})
	}

	return client.Resource(schema.GroupVersionResource{
		Group:    resource.Group,
		Version:  resource.Version,
		Resource: resource.ResourceType,
	}).Namespace(resource.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(metav1.ObjectNameField, name).String(),
	})
}

func (r *SynkTargetReconciler) watchResource(ctx context.Context, resource *synkv1alpha1.Resource, name string, config *rest.Config) {
	// 1. Connect with remote resource
	// 2. Connect with local resource
	// 2. Watch events from remote resource
	homeClient, err := dynamic.NewForConfig(r.Config)
	if err != nil {
		controllerLog.Error(err, "Unable to setup local dynamic client")
		return
	}
	var rch <-chan watch.Event
	var och <-chan watch.Event
	for {
		if rch == nil {
			rch = r.initWatch(ctx, resource, name, config)

			// r.patchCondition(ctx, synk, "Connected", metav1.ConditionTrue, "Connected with remote cluster", "Connected")
		}
		if och == nil {
			och = r.initWatch(ctx, resource, name, r.Config)
		}
		select {
		case <-ctx.Done():
			delete(cancel, getKey(config.Host, resource, name))
			controllerLog.Info("Exit goroutine", "name", getKey(config.Host, resource, name))
			return
		case event, ok := <-rch:
			controllerLog.Info("Remote resource event", "event", event.Type, "timestamp", time.Now().UTC().Format(time.RFC3339Nano))
			if !ok && cancel[getKey(config.Host, resource, name)] == nil {
				controllerLog.Info("Unknown error")
				return
			}
			if event.Type == watch.Deleted {
				controllerLog.Info("Remote resource deleted")
				// r.patchCondition(ctx, synk, "RemoteResourceFound", metav1.ConditionFalse, "Remote resource deleted", "ResourceDeleted")
				continue
			}

			remoteCr, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				controllerLog.Info("Disconnected.")
				rch = nil

				// todo both in 1 patch
				// r.patchCondition(ctx, synk, "Connected", metav1.ConditionFalse, "Remote cluster disconnected", "Disconnected")
				// r.patchCondition(ctx, synk, "RemoteResourceFound", metav1.ConditionFalse, "Remote cluster disconnected", "Disconnected")
				continue
			}

			// r.patchCondition(ctx, synk, "RemoteResourceFound", metav1.ConditionTrue, "Remote resource found", "ResourceFound")

			remoteCrSpec, found, err := unstructured.NestedMap(remoteCr.UnstructuredContent(), "data")
			if !found {
				controllerLog.Info("Spec field not found")
			}
			if err != nil {
				controllerLog.Info("Spec field not of the correct type")
			}

			ownCr, err := homeClient.Resource(schema.GroupVersionResource{
				Group:    resource.Group,
				Version:  resource.Version,
				Resource: resource.ResourceType,
			}).Namespace(resource.Namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					controllerLog.Error(err, "Could not get own resource")
					return
				}
				ownCr = &unstructured.Unstructured{}
				ownCr.SetUnstructuredContent(map[string]interface{}{
					"apiVersion": remoteCr.GetAPIVersion(),
					"kind":       remoteCr.GetKind(),
					"metadata": map[string]interface{}{
						"name":      name,
						"namespace": resource.Namespace,
						"annotations": map[string]interface{}{
							"synk-last-sync":          time.Now().UTC().Format(time.RFC3339Nano),
							"synk-last-remote-update": remoteCr.GetAnnotations()["synk-last-update"],
						},
					},
					"data": remoteCrSpec,
				})

				createdResource, err := homeClient.Resource(schema.GroupVersionResource{
					Group:    resource.Group,
					Version:  resource.Version,
					Resource: resource.ResourceType,
				}).Namespace(resource.Namespace).Create(ctx, ownCr, metav1.CreateOptions{})

				controllerLog.Info("Remote resource updated", "timestamp", createdResource.GetAnnotations()["synk-last-remote-update"])
				controllerLog.Info("Own resource updated", "timestamp", createdResource.GetAnnotations()["synk-last-sync"])

				if err != nil {
					controllerLog.Info("creation own resource failed", "err", err)
				}
				// r.patchCondition(ctx, synk, "OwnResourceFound", metav1.ConditionTrue, "Own resource created", "ResourceCreated")
			} else {

				// r.patchCondition(ctx, synk, "OwnResourceFound", metav1.ConditionTrue, "Own resource found", "ResourceFound")
				ownCrSpec, found, err := unstructured.NestedMap(ownCr.UnstructuredContent(), "data")
				if err != nil {
					controllerLog.Info("Own Spec field not of the correct type")
				}
				if !found {
					controllerLog.Info("Own Spec field not found")
				}
				ownCrJSON, err := json.Marshal(ownCrSpec)
				if err != nil {
					controllerLog.Error(err, "Can't marshal ownCrSpec")
				}
				remoteCrJSON, err := json.Marshal(remoteCrSpec)
				if err != nil {
					controllerLog.Error(err, "Can't marshal remoteCrSpec")
				}

				if string(ownCrJSON) != string(remoteCrJSON) {
					patch := []interface{}{
						map[string]interface{}{
							"op":    "replace",
							"path":  "/data",
							"value": remoteCrSpec,
						},
						map[string]interface{}{
							"op":    "replace",
							"path":  "/metadata/annotations/synk-last-sync",
							"value": time.Now().UTC().Format(time.RFC3339Nano),
						},
						map[string]interface{}{
							"op":    "replace",
							"path":  "/metadata/annotations/synk-last-remote-update",
							"value": remoteCr.GetAnnotations()["synk-last-update"],
						},
					}
					patchPayload, err := json.Marshal(patch)
					if err != nil {
						controllerLog.Error(err, "Can't marshal the patch")
					}

					patchedResource, err := homeClient.Resource(schema.GroupVersionResource{
						Group:    resource.Group,
						Version:  resource.Version,
						Resource: resource.ResourceType,
					}).Namespace(resource.Namespace).Patch(ctx, name, types.JSONPatchType, patchPayload, metav1.PatchOptions{})
					controllerLog.Info("Remote resource updated", "timestamp", patchedResource.GetAnnotations()["synk-last-remote-update"])
					controllerLog.Info("Own resource updated", "timestamp", patchedResource.GetAnnotations()["synk-last-sync"])

					if err != nil {
						controllerLog.Error(err, "Patch ownCr failed")
						return
					}
				}
			}

		case event, ok := <-och:
			controllerLog.Info("Own resource event", "event", event.Type, "timestamp", time.Now().UTC().Format(time.RFC3339Nano))
			if !ok && cancel[getKey(config.Host, resource, name)] == nil {
				controllerLog.Info("Unknown error")
				return
			}
			if event.Type == watch.Deleted {
				controllerLog.Info("Own resource deleted")
				// r.patchCondition(ctx, synk, "OwnResourceFound", metav1.ConditionFalse, "Own resource deleted", "ResourceDeleted")
				continue
			}
		}
	}
}

// func (r *SynkTargetReconciler) patchCondition(ctx context.Context, synk *synkv1alpha1.SynkTarget, conditionType string, status metav1.ConditionStatus, message string, reason string) {
// 	patch := client.MergeFrom(synk.DeepCopy())
// 	for i := range synk.Status.Conditions {
// 		condition := &synk.Status.Conditions[i]
// 		if condition.Type == conditionType {
// 			condition.LastTransitionTime = metav1.NewTime(time.Now())
// 			condition.Status = status
// 			condition.Message = message
// 			condition.Reason = reason
// 		}
// 	}
// 	r.Status().Patch(ctx, synk, patch)

// 	// todo fix: after patch the ownresources are unset
// 	if synk.Spec.OwnResource.Namespace == "" {
// 		synk.Spec.OwnResource.Namespace = synk.Spec.RemoteResource.Namespace
// 	}
// 	if synk.Spec.OwnResource.Name == "" {
// 		synk.Spec.OwnResource.Name = synk.Spec.RemoteResource.Name
// 	}
// }

func createConfig(connection *synkv1alpha1.Connection) (*rest.Config, error) {
	config := &rest.Config{}
	config.Host = connection.Host

	bearerTokenBytes, err := base64.StdEncoding.DecodeString(connection.BearerToken)
	if err != nil {
		controllerLog.Error(err, "Base64 decoding bearertoken failed")
		return nil, err
	}
	config.BearerToken = string(bearerTokenBytes)

	config.CAData, err = base64.StdEncoding.DecodeString(connection.CAData)
	if err != nil {
		controllerLog.Error(err, "Base64 decoding CAData failed")
		return nil, err
	}
	return config, nil
}

func getKey(host string, resource *synkv1alpha1.Resource, name string) string {
	return host + "/" +
		resource.Namespace + "/" +
		resource.Group + "/" +
		resource.Version + "/" +
		resource.ResourceType + "/" +
		name
}

// func createStartCondition(t string, m string, r string) metav1.Condition {
// 	return metav1.Condition{
// 		Type:               t,
// 		Status:             metav1.ConditionFalse,
// 		LastTransitionTime: metav1.NewTime(time.Now()),
// 		Message:            m,
// 		Reason:             r,
// 	}
// }

// SetupWithManager sets up the controller with the Manager.
func (r *SynkTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&synkv1alpha1.SynkTarget{}).
		WithEventFilter(predicateFilterSynkTarget()).
		Complete(r)
}

// Return true to pass it to the reconciler. False will skip the event and it won't be reconciled
func predicateFilterSynkTarget() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			controllerLog.Info("CreatePredicatefilter")
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			synkOld, ok := e.ObjectOld.(*synkv1alpha1.SynkTarget)
			if !ok {
				controllerLog.Info("Could not cast old client.Object to *synkv1alpha1.Synk")
			}

			synkNew, ok := e.ObjectNew.(*synkv1alpha1.SynkTarget)
			if !ok {
				controllerLog.Info("Could not cast new client.Object to *synkv1alpha1.Synk")
			}

			// meta.Genertation does not change with status update. Ignore statusupdates
			if synkOld.GetGeneration() == synkNew.GetGeneration() {
				return false
			}

			// if spec field changes, the goroutine must be deleted and recreated with new params
			for index := range synkOld.Spec.Resources {
				if synkOld.Spec.Resources[index].Names != nil {
					for _, name := range synkOld.Spec.Resources[index].Names {
						cancel[getKey(synkOld.Spec.Connection.Host, &synkOld.Spec.Resources[index], name)]()
					}
				} else {
					cancel[getKey(synkOld.Spec.Connection.Host, &synkOld.Spec.Resources[index], "")]()
				}
			}

			controllerLog.Info("UpdatePredicatefilter")
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			controllerLog.Info("DeletePredicatefilter")
			synk, ok := e.Object.(*synkv1alpha1.SynkTarget)

			if !ok {
				controllerLog.Info("Could not cast client.Object to *synkv1alpha1.Synk")
			}
			for index := range synk.Spec.Resources {
				if synk.Spec.Resources[index].Names != nil {
					for _, name := range synk.Spec.Resources[index].Names {
						cancel[getKey(synk.Spec.Connection.Host, &synk.Spec.Resources[index], name)]()
						delete(cancel, getKey(synk.Spec.Connection.Host, &synk.Spec.Resources[index], name))
					}
				} else {
					cancel[getKey(synk.Spec.Connection.Host, &synk.Spec.Resources[index], "")]()
					delete(cancel, getKey(synk.Spec.Connection.Host, &synk.Spec.Resources[index], ""))
				}
			}
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			controllerLog.Info("GenericPredicatefilter")
			return true
		},
	}
}
