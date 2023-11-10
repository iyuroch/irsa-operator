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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	authv1alpha1 "github.com/iyuroch/irsa-operator/api/v1alpha1"
	"github.com/iyuroch/irsa-operator/controllers/aws"
)

// RoleReconciler reconciles a Role object
type RoleReconciler struct {
	IAMReconciler aws.IIAMReconciler
	client.Client
	Scheme *runtime.Scheme
}

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
	// logger.Info("request for: %s", req.String())

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
		roleLog.Info("Reconciliation run finished", "duration_seconds", time.Since(startTime).Seconds())
	}()

	// reconcile here
	// compare status policy -> if
	// if status policy != policy => CreatePolicy()
	// generatedPolicy := aws.GeneratePolicy(role.Name, role.Namespace, description string, statementEntries *[]StatementEntry)
	// if role.Status.AppliedPolicy !=
	// if status do not have policy arn => create policy and record it in the status
	// if status has policy arn but generated do not match => update policy
	// if deletion => delete policy in aws

	// https://groups.google.com/g/kubernetes-sig-architecture/c/mVGobfD4TpY/m/nkdbkX1iBwAJ
	// TODO: fix me with cluster uid
	policyName := aws.GeneratePolicyName(role.Name, role.Namespace, "uid-cluster")

	roleLog.Info(fmt.Sprintf("policy statements: %s", role.Spec.Statements))

	policyDoc, err := aws.GeneratePolicy(&role.Spec.Statements)
	if err != nil {
		return ctrl.Result{}, err
	}

	roleLog.Info(fmt.Sprintf("creating policy document: %s", *policyDoc))

	if role.Status.PolicyARN == "" {
		roleLog.V(1).Info("creating policy here")
		policyARN, err := r.IAMReconciler.CreatePolicy(ctx, policyName, policyDoc)
		if err != nil {
			roleLog.Error(err, "failed to create policy")
			// TODO: replace with reconcile calls
			return ctrl.Result{}, err
		}
		// refreshing version here so less likely to conflict on update
		if r.Get(ctx, req.NamespacedName, role) != nil {
			return ctrl.Result{}, err
		}
		role.Status.PolicyARN = policyARN
		roleLog.Info("updating status")
		if r.Status().Update(ctx, role) != nil {
			roleLog.Info("updating status failed")
			// in some cases there will be conflict so requeue
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1alpha1.Role{}).
		Complete(r)
}
