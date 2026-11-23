// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Modified by High Five Labs for config-connector-extensions: copied from
// Config Connector's apis/privateca/v1beta1/privatecacertificateauthority_types.go
// (the direct PrivateCACertificateAuthority) and, for the key usage and CA
// options, apis/privateca/v1beta1/types.generated.go, so the spec is the
// one users of privateca.cnrm.cloud.google.com/PrivateCACertificateAuthority
// already write. Changed: group, labels and short names; the project is
// refsv1beta1.ProjectRef (the same schema); every field but the labels and
// userDefinedAccessUrls is immutable by CEL (docs/design.md decision 20),
// since the API takes them only at create (section 6.6, pattern 1); the
// status keeps the authority's state, certificates and access URLs only.
// Written here (foundation item 25), the fields the API takes at create
// that Config Connector's kinds lack: config.x509Config.nameConstraints and
// aiaOcspServers, config.subjectKeyId; and userDefinedAccessUrls.

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/kmsrefs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/privatecarefs"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var PrivateCACertificateAuthorityGVK = GroupVersion.WithKind("PrivateCACertificateAuthority")

// PrivateCACertificateAuthoritySpec defines the desired state of PrivateCACertificateAuthority
// +kcc:spec:proto=google.cloud.security.privateca.v1.CertificateAuthority
type PrivateCACertificateAuthoritySpec struct {
	// Immutable. The project that this resource belongs to: the CA pool's.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef refsv1beta1.ProjectRef `json:"projectRef"`

	// Immutable. The location of this resource: the CA pool's.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location string `json:"location"`

	// Immutable. The PrivateCACertificateAuthority name. If not given, the metadata.name will be used.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	/* Immutable. Required. Immutable. The config used to create a self-signed X.509 certificate or CSR. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateAuthority.config
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="config is immutable"
	Config CertificateConfig `json:"config"`

	/* Immutable. Required. Immutable. Used when issuing certificates for this CertificateAuthority. If this CertificateAuthority is a self-signed CertificateAuthority, this key is also used to sign the self-signed CA certificate. Otherwise, it is used to sign a CSR. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateAuthority.key_spec
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="keySpec is immutable"
	// +kubebuilder:validation:XValidation:rule="has(self.algorithm) != has(self.cloudKmsKeyVersionRef)",message="keySpec takes one of algorithm and cloudKmsKeyVersionRef"
	KeySpec CertificateAuthority_KeyVersionSpec `json:"keySpec"`

	/* Immutable. Required. The desired lifetime of the CA certificate. Used to create the "not_before_time" and "not_after_time" fields inside an X.509 certificate. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateAuthority.lifetime
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="lifetime is immutable"
	Lifetime string `json:"lifetime"`

	/* Immutable. Required. Immutable. The Type of this CertificateAuthority. Possible values: SELF_SIGNED, SUBORDINATE */
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateAuthority.type
	// +kubebuilder:validation:Enum=SELF_SIGNED;SUBORDINATE
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type string `json:"type"`

	/* Immutable. Optional. The StorageBucket where this CertificateAuthority's CA certificate is published. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="gcsBucketRef is immutable"
	GcsBucketRef *storagev1beta1.StorageBucketRef `json:"gcsBucketRef,omitempty"`

	/* Immutable. Optional. The PrivateCACAPool that includes this CertificateAuthority. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="caPoolRef is immutable"
	CaPoolRef privatecarefs.PrivateCACAPoolRef `json:"caPoolRef"`

	// Written here. URLs the CA certificate and CRLs are mirrored to by the
	// user, written into the certificates the authority issues instead of
	// the ones the service publishes (the service does not publish to
	// them). Can change.
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateAuthority.user_defined_access_urls
	// +optional
	UserDefinedAccessUrls *CertificateAuthority_UserDefinedAccessUrls `json:"userDefinedAccessUrls,omitempty"`

	// Written here, after Terraform's skip_grace_period: whether deleting
	// the object deletes the authority at once, without the API's 30-day
	// grace period in which it can be undeleted (and its ID and its CA
	// pool stay in use). Not part of the authority; can change. Unset means
	// false.
	// +optional
	SkipGracePeriod *bool `json:"skipGracePeriod,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.CertificateConfig
type CertificateConfig struct {
	/* Required. Specifies some of the values in a certificate that are related to the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateConfig.subject_config
	SubjectConfig CertificateConfig_SubjectConfig `json:"subjectConfig"`

	/* Required. Describes how some of the technical X.509 fields in a certificate should be populated. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateConfig.x509_config
	X509Config CertificateConfig_X509Config `json:"x509Config"`

	// Written here. The subject key identifier of the authority's
	// certificate, when the default (a hash of the public key) is not
	// wanted.
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateConfig.subject_key_id
	// +optional
	SubjectKeyID *CertificateConfig_KeyID `json:"subjectKeyId,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.CertificateConfig.KeyId
type CertificateConfig_KeyID struct {
	// The key identifier in lowercase hexadecimal, most likely the 160 bit
	// SHA-1 hash of the public key.
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateConfig.KeyId.key_id
	KeyID string `json:"keyId"`
}

// +kcc:proto=google.cloud.security.privateca.v1.CertificateConfig.SubjectConfig
type CertificateConfig_SubjectConfig struct {
	/* Required. Contains distinguished name fields such as the common name, location and organization. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateConfig.SubjectConfig.subject
	Subject Subject `json:"subject"`

	/* Optional. The subject alternative name fields. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateConfig.SubjectConfig.subject_alt_name
	// +optional
	SubjectAltName *SubjectAltNames `json:"subjectAltName,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.Subject
type Subject struct {
	/* The "common name" of the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.Subject.common_name
	// +optional
	CommonName *string `json:"commonName,omitempty"`

	/* The country code of the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.Subject.country_code
	// +optional
	CountryCode *string `json:"countryCode,omitempty"`

	/* The locality or city of the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.Subject.locality
	// +optional
	Locality *string `json:"locality,omitempty"`

	/* The organization of the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.Subject.organization
	// +optional
	Organization *string `json:"organization,omitempty"`

	/* The organizational_unit of the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.Subject.organizational_unit
	// +optional
	OrganizationalUnit *string `json:"organizationalUnit,omitempty"`

	/* The postal code of the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.Subject.postal_code
	// +optional
	PostalCode *string `json:"postalCode,omitempty"`

	/* The province, territory, or regional state of the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.Subject.province
	// +optional
	Province *string `json:"province,omitempty"`

	/* The street address of the subject. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.Subject.street_address
	// +optional
	StreetAddress *string `json:"streetAddress,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.SubjectAltNames
type SubjectAltNames struct {
	/* Contains additional subject alternative name values. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.SubjectAltNames.custom_sans
	// +optional
	CustomSans []CertificateAuthority_X509Extension `json:"customSans,omitempty"`

	/* Contains only valid, fully-qualified host names. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.SubjectAltNames.dns_names
	// +optional
	DNSNames []string `json:"dnsNames,omitempty"`

	/* Contains only valid RFC 2822 E-mail addresses. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.SubjectAltNames.email_addresses
	// +optional
	EmailAddresses []string `json:"emailAddresses,omitempty"`

	/* Contains only valid 32-bit IPv4 addresses or RFC 4291 IPv6 addresses. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.SubjectAltNames.ip_addresses
	// +optional
	IPAddresses []string `json:"ipAddresses,omitempty"`

	/* Contains only valid RFC 3986 URIs. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.SubjectAltNames.uris
	// +optional
	Uris []string `json:"uris,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.X509Parameters
type CertificateConfig_X509Config struct {
	/* Optional. Describes custom X.509 extensions. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.additional_extensions
	// +optional
	AdditionalExtensions []CertificateAuthority_X509Extension `json:"additionalExtensions,omitempty"`

	/* Optional. Describes options in this X509Parameters that are relevant in a CA certificate. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.ca_options
	// +optional
	CaOptions *X509Parameters_CAOptions `json:"caOptions,omitempty"`

	/* Optional. Indicates the intended use for keys that correspond to a certificate. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.key_usage
	// +optional
	KeyUsage *CertificateAuthority_KeyUsage `json:"keyUsage,omitempty"`

	/* Optional. Describes the X.509 certificate policy object identifiers, per https://tools.ietf.org/html/rfc5280#section-4.2.1.4. */
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.policy_ids
	// +optional
	PolicyIds []CertificateAuthority_ObjectID `json:"policyIds,omitempty"`

	// Written here. Optional. Describes Online Certificate Status Protocol
	// (OCSP) endpoint addresses that appear in the "Authority Information
	// Access" extension in the certificate.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.aia_ocsp_servers
	// +optional
	AiaOcspServers []string `json:"aiaOcspServers,omitempty"`

	// Written here. Optional. Describes the X.509 name constraints
	// extension: the names the authority's certificates may (permitted)
	// and may not (excluded) carry.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.name_constraints
	// +optional
	NameConstraints *X509Parameters_NameConstraints `json:"nameConstraints,omitempty"`
}

type CertificateAuthority_X509Extension struct {
	/* Required. The OID for this X.509 extension. */
	ObjectID CertificateAuthority_ObjectID `json:"objectId"`

	/* Optional. Indicates whether or not this extension is critical (i.e., if the client does not know how to handle this extension, the client should consider this to be an error). */
	// +optional
	Critical *bool `json:"critical,omitempty"`

	/* Required. The value of this X.509 extension. */
	Value string `json:"value"`
}

type CertificateAuthority_ObjectID struct {
	/* Required. The parts of an OID path. The most significant parts of the path come first. */
	ObjectIDPath []int64 `json:"objectIdPath"`
}

type CertificateAuthority_KeyUsage struct {
	/* Describes high-level ways in which a key may be used. */
	// +optional
	BaseKeyUsage *KeyUsage_KeyUsageOptions `json:"baseKeyUsage,omitempty"`

	/* Detailed scenarios in which a key may be used. */
	// +optional
	ExtendedKeyUsage *KeyUsage_ExtendedKeyUsageOptions `json:"extendedKeyUsage,omitempty"`

	/* Used to describe extended key usages that are not listed in the KeyUsage.ExtendedKeyUsageOptions message. */
	// +optional
	UnknownExtendedKeyUsages []CertificateAuthority_ObjectID `json:"unknownExtendedKeyUsages,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.KeyUsage.ExtendedKeyUsageOptions
type KeyUsage_ExtendedKeyUsageOptions struct {
	// Corresponds to OID 1.3.6.1.5.5.7.3.1. Officially described as "TLS WWW
	//  server authentication", though regularly used for non-WWW TLS.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.ExtendedKeyUsageOptions.server_auth
	ServerAuth *bool `json:"serverAuth,omitempty"`

	// Corresponds to OID 1.3.6.1.5.5.7.3.2. Officially described as "TLS WWW
	//  client authentication", though regularly used for non-WWW TLS.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.ExtendedKeyUsageOptions.client_auth
	ClientAuth *bool `json:"clientAuth,omitempty"`

	// Corresponds to OID 1.3.6.1.5.5.7.3.3. Officially described as "Signing of
	//  downloadable executable code client authentication".
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.ExtendedKeyUsageOptions.code_signing
	CodeSigning *bool `json:"codeSigning,omitempty"`

	// Corresponds to OID 1.3.6.1.5.5.7.3.4. Officially described as "Email
	//  protection".
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.ExtendedKeyUsageOptions.email_protection
	EmailProtection *bool `json:"emailProtection,omitempty"`

	// Corresponds to OID 1.3.6.1.5.5.7.3.8. Officially described as "Binding
	//  the hash of an object to a time".
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.ExtendedKeyUsageOptions.time_stamping
	TimeStamping *bool `json:"timeStamping,omitempty"`

	// Corresponds to OID 1.3.6.1.5.5.7.3.9. Officially described as "Signing
	//  OCSP responses".
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.ExtendedKeyUsageOptions.ocsp_signing
	OcspSigning *bool `json:"ocspSigning,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions
type KeyUsage_KeyUsageOptions struct {
	// The key may be used for digital signatures.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.digital_signature
	DigitalSignature *bool `json:"digitalSignature,omitempty"`

	// The key may be used for cryptographic commitments. Note that this may
	//  also be referred to as "non-repudiation".
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.content_commitment
	ContentCommitment *bool `json:"contentCommitment,omitempty"`

	// The key may be used to encipher other keys.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.key_encipherment
	KeyEncipherment *bool `json:"keyEncipherment,omitempty"`

	// The key may be used to encipher data.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.data_encipherment
	DataEncipherment *bool `json:"dataEncipherment,omitempty"`

	// The key may be used in a key agreement protocol.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.key_agreement
	KeyAgreement *bool `json:"keyAgreement,omitempty"`

	// The key may be used to sign certificates.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.cert_sign
	CertSign *bool `json:"certSign,omitempty"`

	// The key may be used sign certificate revocation lists.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.crl_sign
	CrlSign *bool `json:"crlSign,omitempty"`

	// The key may be used to encipher only.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.encipher_only
	EncipherOnly *bool `json:"encipherOnly,omitempty"`

	// The key may be used to decipher only.
	// +kcc:proto:field=google.cloud.security.privateca.v1.KeyUsage.KeyUsageOptions.decipher_only
	DecipherOnly *bool `json:"decipherOnly,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.X509Parameters.CaOptions
type X509Parameters_CAOptions struct {
	// Optional. Refers to the "CA" boolean field in the X.509 extension.
	//  When this value is missing, the basic constraints extension will be
	//  omitted from the certificate.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.CaOptions.is_ca
	IsCA *bool `json:"isCA,omitempty"`

	// Optional. Refers to the path length constraint field in the X.509
	//  extension. For a CA certificate, this value describes the depth of
	//  subordinate CA certificates that are allowed. If this value is less than
	//  0, the request will fail. If this value is missing, the max path length
	//  will be omitted from the certificate.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.CaOptions.max_issuer_path_length
	MaxIssuerPathLength *int32 `json:"maxIssuerPathLength,omitempty"`
}

type CertificateAuthority_KeyVersionSpec struct {
	// Immutable. The resource name for an existing Cloud KMS CryptoKeyVersion in the format
	// `projects/{project}/locations/{location}/keyRings/{keyRing}/cryptoKeys/{key}/cryptoKeyVersions/{version}`.
	// +optional
	CloudKmsKeyVersionRef *kmsrefs.KMSCryptoKeyVersionRef `json:"cloudKmsKeyVersionRef,omitempty"`

	/* Immutable. The algorithm to use for creating a managed Cloud KMS key for a for a simplified experience. All managed keys will be have their ProtectionLevel as `HSM`. Possible values: SIGN_HASH_ALGORITHM_UNSPECIFIED, RSA_PSS_2048_SHA256, RSA_PSS_3072_SHA256, RSA_PSS_4096_SHA256, RSA_PKCS1_2048_SHA256, RSA_PKCS1_3072_SHA256, RSA_PKCS1_4096_SHA256, EC_P256_SHA256, EC_P384_SHA384 */
	// +optional
	Algorithm *string `json:"algorithm,omitempty"`
}

// +kcc:proto=google.cloud.security.privateca.v1.CertificateAuthority.UserDefinedAccessUrls
type CertificateAuthority_UserDefinedAccessUrls struct {
	// Optional. URLs where the CA certificate is mirrored, written into the
	// "Authority Information Access" extension of the certificates issued.
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateAuthority.UserDefinedAccessUrls.aia_issuing_certificate_urls
	// +optional
	AiaIssuingCertificateUrls []string `json:"aiaIssuingCertificateUrls,omitempty"`

	// Optional. URLs where the CRLs are mirrored, written into the "CRL
	// Distribution Points" extension of the certificates issued.
	// +kcc:proto:field=google.cloud.security.privateca.v1.CertificateAuthority.UserDefinedAccessUrls.crl_access_urls
	// +optional
	CrlAccessUrls []string `json:"crlAccessUrls,omitempty"`
}

// PrivateCACertificateAuthorityStatus defines the config connector machine state of PrivateCACertificateAuthority
type PrivateCACertificateAuthorityStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// A unique specifier for the PrivateCACertificateAuthority resource in GCP.
	ExternalRef *string `json:"externalRef,omitempty"`

	/* Output only. URLs for accessing content published by this CA, such as the CA certificate and CRLs. */
	AccessUrls *CertificateAuthority_AccessUrls `json:"accessUrls,omitempty"`

	/* Output only. This CertificateAuthority's certificate chain, including the current CertificateAuthority's certificate. Ordered such that the root issuer is the final element (consistent with RFC 5246). For a self-signed CA, this will only list the current CertificateAuthority's certificate. */
	PemCaCertificates []string `json:"pemCaCertificates,omitempty"`

	/* Output only. The State for this CertificateAuthority. Possible values: ENABLED, DISABLED, STAGED, AWAITING_USER_ACTIVATION, DELETED */
	State *string `json:"state,omitempty"`

	/* Output only. The CaPool.Tier of the CaPool that includes this CertificateAuthority. Possible values: ENTERPRISE, DEVOPS */
	Tier *string `json:"tier,omitempty"`
}

type CertificateAuthority_AccessUrls struct {
	/* The URL where this CertificateAuthority's CA certificate is published. This will only be set for CAs that have been activated. */
	// +optional
	CaCertificateAccessUrl *string `json:"caCertificateAccessUrl,omitempty"`

	/* The URLs where this CertificateAuthority's CRLs are published. This will only be set for CAs that have been activated. */
	// +optional
	CrlAccessUrls []string `json:"crlAccessUrls,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflprivatecacertificateauthority;hflprivatecacertificateauthorities
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// PrivateCACertificateAuthority is a Certificate Authority Service
// certificate authority, made by this kind with the fields Config
// Connector's lacks that the API takes only at create. Use one kind or the
// other for an authority, never both.
// +k8s:openapi-gen=true
type PrivateCACertificateAuthority struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   PrivateCACertificateAuthoritySpec   `json:"spec,omitempty"`
	Status PrivateCACertificateAuthorityStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// PrivateCACertificateAuthorityList contains a list of PrivateCACertificateAuthority
type PrivateCACertificateAuthorityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrivateCACertificateAuthority `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrivateCACertificateAuthority{}, &PrivateCACertificateAuthorityList{})
}
