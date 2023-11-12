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

package v1alpha1

import (
	"github.com/iyuroch/irsa-operator/controllers/aws"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RoleSpec defines the desired state of Role
type RoleSpec struct {
	// +kubebuilder:validation:MinItems:=1
	Statements []aws.StatementEntry `json:"statements"`
	// provide oidc provider you created for eks cluster
	// https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html
	OIDCProvider string `json:"oidcprovider"`
}

// RoleStatus defines the observed state of Role
type RoleStatus struct {
	Reconciled bool `json:"reconciled,omitempty"`
	// stores marshaled last applied policy document
	AppliedPolicyDocument string `json:"appliedpolicydocument,omitempty"`
	// stores role name which is sa + namespace + cluster name + md5 hash
	RoleName        string `json:"rolename,omitempty"`
	PolicyARN       string `json:"policyarn,omitempty"`
	RolePolicyBound bool   `json:"bound,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:validation:Required

// Role is the Schema for the roles API
type Role struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleSpec   `json:"spec"`
	Status RoleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RoleList contains a list of Role
type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Role `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Role{}, &RoleList{})
}
