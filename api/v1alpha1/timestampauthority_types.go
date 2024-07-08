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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TimestampAuthoritySpec defines the desired state of TimestampAuthority
type TimestampAuthoritySpec struct {
	// Define whether you want to export service or not
	ExternalAccess ExternalAccess `json:"externalAccess,omitempty"`
	// Signer configuration
	Signer TimestampAuthoritySigner `json:"signer,omitempty"`
}

type TimestampAuthoritySigner struct {
	// Configuration for the Certificate Chain
	CertificateChain CertificateChain `json:"certificateChain,omitempty"`
	// Configuration for file-based signer
	//+optional
	FileSigner *FileSigner `json:"fileSigner,omitempty"`
	// Configuration for KMS based signer
	//+optional
	KmsSigner *KmsSigner `json:"kmsSigner,omitempty"`
	//Configuration for Tink based signer
	//+optional
	TinkSigner *TinkSigner `json:"tinkSigner,omitempty"`
}

type CertificateChain struct {
	//Reference to the certificate chain
	//+optional
	CertificateChainRef *SecretKeySelector `json:"certificateChainRef,omitempty"`
	//Root Certificate Authority Config
	//+optional
	RootCA TsaCertificateAuthority `json:"rootCA,omitempty"`
	//Intermediate Certificate Authority Config
	//+optional
	IntermediateCA TsaCertificateAuthority `json:"intermediateCA,omitempty"`
}

type TsaCertificateAuthority struct {
	// CommonName specifies the common name for the TimeStampAuthorities cert chain.
	// If not provided, the common name will default to the host name.
	//+optional
	CommonName string `json:"commonName,omitempty"`
	//+optional
	//OrganizationName specifies the Organization Name for the TimeStampAuthorities cert chain.
	OrganizationName string `json:"organizationName,omitempty"`
	//+optional
	//Organization Email specifies the Organization Email for the TimeStampAuthorities cert chain.
	OrganizationEmail string `json:"organizationEmail,omitempty"`
	// Password to decrypt the signer's root private key
	//+optional
	PasswordRef *SecretKeySelector `json:"passwordRef,omitempty"`
	// Reference to the signer's root private key
	//+optional
	PrivateKeyRef *SecretKeySelector `json:"privateKeyRef,omitempty"`
}

type FileSigner struct {
	// Password to decrypt the signer's root private key
	//+optional
	PasswordRef *SecretKeySelector `json:"passwordRef,omitempty"`
	// Reference to the signer's root private key
	//+optional
	PrivateKeyRef *SecretKeySelector `json:"privateKeyRef,omitempty"`
}

type KmsSigner struct {
	// KMS key for signing timestamp responses. Valid options include: [gcpkms://resource, azurekms://resource, hashivault://resource, awskms://resource]
	//+optional
	KmsKeyResource string `json:"kmsKeyResource,omitempty"`
	// Configuration for authentication for key managment services
	//+optional
	KmsAuthConfig KmsAuthConfig `json:"kmsAuthConfig,omitempty"`
}

type TinkSigner struct {
	//KMS key for signing timestamp responses for Tink keysets. Valid options include: [gcp-kms://resource, aws-kms://resource, hcvault://]"
	//+optional
	TinkKeyResource string `json:"tinkKeyResource,omitempty"`
	//+optional
	//Path to KMS-encrypted keyset for Tink, decrypted by TinkKeyResource
	TinkKeysetRef *SecretKeySelector `json:"tinkKeysetRef,omitempty"`
	//Authentication token for Hashicorp Vault API calls
	//+optional
	TinkHcvaultTokenRef *SecretKeySelector `json:"tinkHcvaultTokenRef,omitempty"`
	// Configuration for authentication for key managment services
	//+optional
	KmsAuthConfig KmsAuthConfig `json:"kmsAuthConfig,omitempty"`
}

func (i *TimestampAuthority) GetConditions() []metav1.Condition {
	return i.Status.Conditions
}

func (i *TimestampAuthority) SetCondition(newCondition metav1.Condition) {
	meta.SetStatusCondition(&i.Status.Conditions, newCondition)
}

// TimestampAuthorityStatus defines the observed state of TimestampAuthority
type TimestampAuthorityStatus struct {
	Signer *TimestampAuthoritySigner `json:"signer,omitempty"`
	Url    string                    `json:"url,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TimestampAuthority is the Schema for the timestampauthorities API
type TimestampAuthority struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TimestampAuthoritySpec   `json:"spec,omitempty"`
	Status TimestampAuthorityStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TimestampAuthorityList contains a list of TimestampAuthority
type TimestampAuthorityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TimestampAuthority `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TimestampAuthority{}, &TimestampAuthorityList{})
}