/*
Copyright 2023.

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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	authv1alpha1 "github.com/iyuroch/irsa-operator/api/v1alpha1"
	"github.com/iyuroch/irsa-operator/controllers/aws"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RoleReconciler reconciles a Role object
type RoleReconciler struct {
	IAMReconciler aws.IIAMReconciler
	client.Client
	Scheme       *runtime.Scheme
	InstanceId   string
	OIDCProvider string
}

const finalizer = "roles.auth.irsa.aws/finalizer"

// how many of characters from resource uid to use during role and policy name generation
const uidChars = 8

//+kubebuilder:rbac:groups=auth.irsa.aws,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=auth.irsa.aws,resources=roles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=auth.irsa.aws,resources=roles/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Role object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *RoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	role := &authv1alpha1.Role{}

	err := r.Get(ctx, req.NamespacedName, role)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	roleLog := logger.WithValues("namespace", role.Namespace, "role", role.Name)

	// Printout the duration of the reconciliation, independent if the reconciliation was successful or had an error.
	startTime := time.Now()
	defer func() {
		roleLog.Info("reconciliation run finished", "duration_seconds", time.Since(startTime).Seconds())
	}()

	awsResourceName := aws.GenerateResourceName(role.Name, role.Namespace, string(role.UID)[:uidChars]+r.InstanceId)
	roleLog.Info(fmt.Sprintf("aws resource name: %s", awsResourceName))

	roleLog.Info(fmt.Sprintf("policy statements: %s", role.Spec.Statements))

	policyDoc, err := aws.GeneratePolicyDocument(&role.Spec.Statements)
	if err != nil {
		return ctrl.Result{}, err
	}
	roleLog.Info(fmt.Sprintf("policy document: %s", *policyDoc))
	var oidcProvider string
	switch {
	case role.Spec.OIDCProvider != "":
		oidcProvider = role.Spec.OIDCProvider
	case r.OIDCProvider != "":
		oidcProvider = r.OIDCProvider
	default:
		return ctrl.Result{}, fmt.Errorf("there is no global or role-specific OIDC provider")
	}

	// add finalizer if doesn't exist
	if !controllerutil.ContainsFinalizer(role, finalizer) {
		roleLog.Info("adding finalizer to the object")
		controllerutil.AddFinalizer(role, finalizer)
		if err := r.Update(ctx, role); err != nil {
			return ctrl.Result{}, err
		}
	}

	// it's being deleted right now
	if !role.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.delete(ctx, role, roleLog)
	}

	// it means that policy not created
	if role.Status.PolicyARN == "" {
		roleLog.Info("creating policy")
		policyARN, err := r.IAMReconciler.CreatePolicy(ctx, awsResourceName, policyDoc)
		if err != nil {
			roleLog.Error(err, "failed to create policy")
			return ctrl.Result{}, err
		}
		role.Status.PolicyARN = policyARN
		roleLog.Info("updating status with policyARN")
		if err := r.Status().Update(ctx, role); err != nil {
			roleLog.Error(err, "updating status failed")
			return ctrl.Result{}, err
		}
	}

	roleLog.Info("checking if policy doc matches state")
	if role.Status.AppliedPolicyDocument != *policyDoc {
		roleLog.Info("updating policy document")
		if r.IAMReconciler.UpdatePolicyDocument(ctx, role.Status.PolicyARN, policyDoc) != nil {
			roleLog.Error(err, "cannot update policy document")
			return ctrl.Result{}, err
		}
		role.Status.AppliedPolicyDocument = *policyDoc
		if err := r.Status().Update(ctx, role); err != nil {
			roleLog.Error(err, "updating applied policy document failed")
			return ctrl.Result{}, err
		}
	}

	roleLog.Info("checking if role name matches state")
	if role.Status.RoleName == "" {
		roleLog.Info("creating role")
		if err := r.IAMReconciler.CreateRole(ctx, role.Namespace, role.Name,
			awsResourceName, oidcProvider); err != nil {
			roleLog.Error(err, "creating role failed")
			return ctrl.Result{}, err
		}
		role.Status.RoleName = awsResourceName
		if err := r.Status().Update(ctx, role); err != nil {
			roleLog.Error(err, "updating rolename failed")
			return ctrl.Result{}, err
		}
	}

	roleLog.Info("checking policy is bound to role")
	if !role.Status.RolePolicyBound {
		roleLog.Info("binding policy to role")
		if r.IAMReconciler.RolePolicyBind(ctx, role.Status.PolicyARN, role.Status.RoleName) != nil {
			return ctrl.Result{}, err
		}
		role.Status.RolePolicyBound = true
		if err := r.Status().Update(ctx, role); err != nil {
			roleLog.Error(err, "updating role policy bind failed")
			return ctrl.Result{}, err
		}
	}

	roleLog.Info("checking service account created")
	if !role.Status.ServiceAccountCreated {
		roleLog.Info("creating service account")
		awsAccountId, err := aws.GetAccountId()
		if err != nil {
			return ctrl.Result{}, err
		}
		roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", awsAccountId, role.Status.RoleName)
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      role.Name,
				Namespace: role.Namespace,
				Annotations: map[string]string{
					"eks.amazonaws.com/role-arn": roleArn,
				},
			},
		}
		if err := ctrl.SetControllerReference(role, sa, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Client.Create(ctx, sa); err != nil {
			return ctrl.Result{}, err
		}
		role.Status.ServiceAccountCreated = true
		if err := r.Status().Update(ctx, role); err != nil {
			roleLog.Error(err, "updating service account status failed")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *RoleReconciler) delete(ctx context.Context, role *authv1alpha1.Role, roleLog logr.Logger) (result ctrl.Result, err error) {
	roleLog.Info("deleting object")
	if controllerutil.ContainsFinalizer(role, finalizer) {
		roleLog.Info("deleting external dependencies")
		if role.Status.RolePolicyBound {
			err = r.IAMReconciler.DetachPolicyFromRole(ctx, role.Status.RoleName, role.Status.PolicyARN)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		if role.Status.RoleName != "" {
			roleLog.Info("deleting role")
			err = r.IAMReconciler.DeleteRole(ctx, role.Status.RoleName)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		if role.Status.PolicyARN != "" {
			roleLog.Info("deleting policy")
			err = r.IAMReconciler.DeletePolicy(ctx, role.Status.PolicyARN)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		controllerutil.RemoveFinalizer(role, finalizer)
		err := r.Update(ctx, role)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1alpha1.Role{}).
		Owns(&corev1.ServiceAccount{}).
		Complete(r)
}
