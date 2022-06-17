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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	synkv1alpha1 "github.com/VerwaerdeWim/Synk/api/v1alpha1"
)

const synkSourceFinalizer = "synksource.synk.io/finalizer"

var (
	synkSourceLog = ctrl.Log.WithName("SynkSource controller")
	roles         = make(map[string]*rbacv1.Role)
)

// SynkSourceReconciler reconciles a SynkSource object
type SynkSourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=synk.io,resources=synksources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=synk.io,resources=synksources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=synk.io,resources=synksources/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SynkSource object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *SynkSourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	synkSourceLog.Info("Reconcile")

	synkSource := &synkv1alpha1.SynkSource{}
	err := r.Get(ctx, req.NamespacedName, synkSource)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			synkSourceLog.Info("SynkSource resource is deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	isSynkSourceMarkedToBeDeleted := synkSource.GetDeletionTimestamp() != nil
	if isSynkSourceMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(synkSource, synkSourceFinalizer) {

			if err := r.finalizeSynkSource(ctx, synkSource); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(synkSource, synkSourceFinalizer)
			err := r.Update(ctx, synkSource)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(synkSource, synkSourceFinalizer) {
		controllerutil.AddFinalizer(synkSource, synkSourceFinalizer)
		err = r.Update(ctx, synkSource)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update synksource
	if synkSource.Spec.Connection != nil {

		newRoles := make(map[string]*rbacv1.Role)
		newRoleBindings := make(map[string]*rbacv1.RoleBinding)

		for _, resource := range synkSource.Spec.Resources {
			if newRoles[resource.Namespace] != nil {
				role := newRoles[resource.Namespace]

				i := 0
				for i < len(role.Rules) {
					if role.Rules[i].APIGroups[0]+role.Rules[i].Resources[0] == resource.Group+resource.ResourceType {
						role.Rules[i].ResourceNames = append(role.Rules[i].ResourceNames, resource.Names...)
						break
					}
					i++
				}

				if i == len(role.Rules) {
					role.Rules = append(role.Rules, rbacv1.PolicyRule{
						Verbs:         []string{"watch"},
						APIGroups:     []string{resource.Group},
						Resources:     []string{resource.ResourceType},
						ResourceNames: resource.Names,
					})
				}

			} else {
				role := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      synkSource.Name,
						Namespace: resource.Namespace,
					},
					Rules: []rbacv1.PolicyRule{{
						Verbs:         []string{"watch"},
						APIGroups:     []string{resource.Group},
						Resources:     []string{resource.ResourceType},
						ResourceNames: resource.Names,
					}},
				}
				newRoles[resource.Namespace] = role
			}
			if newRoleBindings[resource.Namespace] == nil {
				rolebinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      synkSource.Name,
						Namespace: resource.Namespace,
					},
					Subjects: []rbacv1.Subject{{
						Kind:      "ServiceAccount",
						Name:      synkSource.Name,
						Namespace: synkSource.Namespace,
					}},
					RoleRef: rbacv1.RoleRef{
						Kind:     "Role",
						Name:     synkSource.Name,
						APIGroup: "rbac.authorization.k8s.io",
					},
				}
				newRoleBindings[resource.Namespace] = rolebinding
			}
		}

		var rolesToRemove []string
		for ns := range roles {
			_, found := newRoles[ns]
			if !found {
				rolesToRemove = append(rolesToRemove, ns)
			}
		}
		roles = newRoles

		if len(rolesToRemove) > 0 {
			for _, ns := range rolesToRemove {
				roleBinding := &rbacv1.RoleBinding{}
				err := r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: ns}, roleBinding)
				if err != nil {
					synkSourceLog.Info("Could not get the rolebinding", "namespace", ns)
				} else {
					_ = r.Delete(ctx, roleBinding)
				}

				role := &rbacv1.Role{}
				err = r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: ns}, role)
				if err != nil {
					synkSourceLog.Info("Could not get the role", "namespace", ns)
				} else {
					_ = r.Delete(ctx, role)
				}
			}
		}

		for _, role := range newRoles {
			oldRole := &rbacv1.Role{}
			err := r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: role.Namespace}, oldRole)
			if err != nil {
				if errors.IsNotFound(err) {
					synkSourceLog.Info("Role does not exist, need to create a role", "namespace", role.Namespace)
					err := r.Create(ctx, role)
					if err != nil {
						synkSourceLog.Info("Role creation failed", "namespace", role.Namespace)
					}
				}
			} else {
				patch := client.MergeFrom(oldRole.DeepCopy())
				oldRole.Rules = role.Rules
				err = r.Patch(ctx, oldRole, patch)
				if err != nil {
					synkSourceLog.Info("Patch of role failed", "namespace", role.Namespace)
				}
			}
		}

		for _, roleBinding := range newRoleBindings {
			oldRoleBinding := &rbacv1.RoleBinding{}
			err := r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: roleBinding.Namespace}, oldRoleBinding)
			if err != nil {
				if errors.IsNotFound(err) {
					synkSourceLog.Info("Could not get the rolebinding", "namespace", roleBinding.Namespace)
					err = r.Create(ctx, roleBinding)
					if err != nil {
						synkSourceLog.Info("RoleBinding creation failed", "namespace", roleBinding.Namespace)
					}
				}
			}

		}
		return ctrl.Result{}, nil
	}

	// Create synksource
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      synkSource.Name,
			Namespace: synkSource.Namespace,
		},
	}
	err = r.Create(ctx, sa)
	if err != nil {
		synkSourceLog.Info("Service account creation failed")
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      synkSource.Name,
			Namespace: synkSource.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": synkSource.Name,
			},
		},
		Type: "kubernetes.io/service-account-token",
	}
	err = r.Create(ctx, secret)
	if err != nil {
		synkSourceLog.Info("Secret creation failed")
	}

	roles := make(map[string]*rbacv1.Role)
	roleBindings := make(map[string]*rbacv1.RoleBinding)

	for _, resource := range synkSource.Spec.Resources {
		if roles[resource.Namespace] != nil {
			role := roles[resource.Namespace]

			i := 0
			for i < len(role.Rules) {
				if role.Rules[i].APIGroups[0]+role.Rules[i].Resources[0] == resource.Group+resource.ResourceType {
					role.Rules[i].ResourceNames = append(role.Rules[i].ResourceNames, resource.Names...)
					break
				}
				i++
			}

			if i == len(role.Rules) {
				role.Rules = append(role.Rules, rbacv1.PolicyRule{
					Verbs:         []string{"watch"},
					APIGroups:     []string{resource.Group},
					Resources:     []string{resource.ResourceType},
					ResourceNames: resource.Names,
				})
			}

		} else {
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      synkSource.Name,
					Namespace: resource.Namespace,
				},
				Rules: []rbacv1.PolicyRule{{
					Verbs:         []string{"watch"},
					APIGroups:     []string{resource.Group},
					Resources:     []string{resource.ResourceType},
					ResourceNames: resource.Names,
				}},
			}
			roles[resource.Namespace] = role
		}
		if roleBindings[resource.Namespace] == nil {
			rolebinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      synkSource.Name,
					Namespace: resource.Namespace,
				},
				Subjects: []rbacv1.Subject{{
					Kind:      "ServiceAccount",
					Name:      synkSource.Name,
					Namespace: synkSource.Namespace,
				}},
				RoleRef: rbacv1.RoleRef{
					Kind:     "Role",
					Name:     synkSource.Name,
					APIGroup: "rbac.authorization.k8s.io",
				},
			}
			roleBindings[resource.Namespace] = rolebinding
		}
	}
	for _, role := range roles {
		err := r.Create(ctx, role)
		if err != nil {
			synkSourceLog.Info("Role creation failed", "namespace", role.Namespace)
		}
	}

	for _, roleBinding := range roleBindings {
		err = r.Create(ctx, roleBinding)
		if err != nil {
			synkSourceLog.Info("RoleBinding creation failed", "namespace", roleBinding.Namespace)
		}
	}

	err = r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: synkSource.Namespace}, secret)
	if err != nil {
		synkSourceLog.Info("Could not get the secret")
	}

	if secret.Data == nil {
		synkSourceLog.Info("Secret contains no data")
	} else {
		synkSource.Spec.Connection = &synkv1alpha1.Connection{
			Host:        ctrl.GetConfigOrDie().Host,
			BearerToken: base64.StdEncoding.EncodeToString(secret.Data["token"]),
			CAData:      base64.StdEncoding.EncodeToString(secret.Data["ca.crt"]),
		}

		err = r.Update(ctx, synkSource)
		if err != nil {
			synkSourceLog.Info("SynkSource update with connection info failed")
		}
	}

	return ctrl.Result{}, nil
}

func (r *SynkSourceReconciler) finalizeSynkSource(ctx context.Context, synkSource *synkv1alpha1.SynkSource) error {
	for _, resource := range synkSource.Spec.Resources {
		roleBinding := &rbacv1.RoleBinding{}
		err := r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: resource.Namespace}, roleBinding)
		if err != nil {
			synkSourceLog.Info("Could not get the rolebinding", "namespace", resource.Namespace)
		} else {
			_ = r.Delete(ctx, roleBinding)
		}

		role := &rbacv1.Role{}
		err = r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: resource.Namespace}, role)
		if err != nil {
			synkSourceLog.Info("Could not get the role", "namespace", resource.Namespace)
		} else {
			_ = r.Delete(ctx, role)
		}

	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: synkSource.Namespace}, secret)
	if err != nil {
		synkSourceLog.Info("Could not get the secret")
	} else {
		_ = r.Delete(ctx, secret)
	}

	sa := &corev1.ServiceAccount{}
	err = r.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: synkSource.Namespace}, sa)
	if err != nil {
		synkSourceLog.Info("Could not get the service account")
	} else {
		_ = r.Delete(ctx, sa)
	}
	synkSourceLog.Info("Successfully finalized SynkSource")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SynkSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&synkv1alpha1.SynkSource{}).
		WithEventFilter(predicateFilterSynkSource()).
		Complete(r)
}

// Return true to pass it to the reconciler. False will skip the event and it won't be reconciled
func predicateFilterSynkSource() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			synkSourceLog.Info("CreatePredicatefilter")
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			synkSourceOld, ok := e.ObjectOld.(*synkv1alpha1.SynkSource)
			if !ok {
				synkSourceLog.Info("Could not cast old client.Object to *synkv1alpha1.SynkSource")
			}

			synkSourceNew, ok := e.ObjectNew.(*synkv1alpha1.SynkSource)
			if !ok {
				synkSourceLog.Info("Could not cast new client.Object to *synkv1alpha1.SynkSource")
			}

			// meta.Genertation does not change with status update. Ignore statusupdates
			if synkSourceOld.GetGeneration() == synkSourceNew.GetGeneration() {
				return false
			}

			// Ignore when connection is added
			if synkSourceOld.Spec.Connection == nil {
				return false
			}

			synkSourceLog.Info("UpdatePredicatefilter")
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			synkSourceLog.Info("DeletePredicatefilter")
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			synkSourceLog.Info("GenericPredicatefilter")
			return true
		},
	}
}
