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

package privateca

// Modified by High Five Labs for config-connector-extensions: the spec's
// mapping to the API's CertificateAuthority, copied from Config Connector's
// pkg/controller/direct/privateca/mapper.go and mapper.generated.go (the
// ToProto functions of PrivateCACertificateAuthority's spec). Changed: our
// KRM types; only ToProto (the kind compares nothing it made at create);
// written here, the fields upstream's mapping lacks: config.subjectKeyId,
// x509Config.aiaOcspServers and nameConstraints, and userDefinedAccessUrls.

import (
	pb "cloud.google.com/go/security/privateca/apiv1/privatecapb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/privateca/v1alpha1"
)

func PrivateCACertificateAuthoritySpec_ToProto(mapCtx *direct.MapContext, in *krm.PrivateCACertificateAuthoritySpec) *pb.CertificateAuthority {
	if in == nil {
		return nil
	}
	out := &pb.CertificateAuthority{}
	out.Type = direct.Enum_ToProto[pb.CertificateAuthority_Type](mapCtx, &in.Type)
	out.Config = CertificateConfig_ToProto(mapCtx, &in.Config)
	out.Lifetime = direct.StringDuration_ToProto(mapCtx, &in.Lifetime)
	out.KeySpec = CertificateAuthority_KeyVersionSpec_ToProto(mapCtx, &in.KeySpec)
	if in.GcsBucketRef != nil {
		out.GcsBucket = in.GcsBucketRef.External
	}
	out.UserDefinedAccessUrls = CertificateAuthority_UserDefinedAccessUrls_ToProto(mapCtx, in.UserDefinedAccessUrls)
	return out
}

func CertificateConfig_ToProto(mapCtx *direct.MapContext, in *krm.CertificateConfig) *pb.CertificateConfig {
	if in == nil {
		return nil
	}
	out := &pb.CertificateConfig{}
	out.SubjectConfig = CertificateConfig_SubjectConfig_ToProto(mapCtx, &in.SubjectConfig)
	out.X509Config = CertificateConfig_X509Config_ToProto(mapCtx, &in.X509Config)
	if in.SubjectKeyID != nil {
		out.SubjectKeyId = &pb.CertificateConfig_KeyId{KeyId: in.SubjectKeyID.KeyID}
	}
	return out
}

func CertificateConfig_SubjectConfig_ToProto(mapCtx *direct.MapContext, in *krm.CertificateConfig_SubjectConfig) *pb.CertificateConfig_SubjectConfig {
	if in == nil {
		return nil
	}
	out := &pb.CertificateConfig_SubjectConfig{}
	out.Subject = Subject_ToProto(mapCtx, &in.Subject)
	out.SubjectAltName = SubjectAltNames_ToProto(mapCtx, in.SubjectAltName)
	return out
}

func Subject_ToProto(mapCtx *direct.MapContext, in *krm.Subject) *pb.Subject {
	if in == nil {
		return nil
	}
	out := &pb.Subject{}
	out.CommonName = direct.ValueOf(in.CommonName)
	out.CountryCode = direct.ValueOf(in.CountryCode)
	out.Organization = direct.ValueOf(in.Organization)
	out.OrganizationalUnit = direct.ValueOf(in.OrganizationalUnit)
	out.Locality = direct.ValueOf(in.Locality)
	out.Province = direct.ValueOf(in.Province)
	out.StreetAddress = direct.ValueOf(in.StreetAddress)
	out.PostalCode = direct.ValueOf(in.PostalCode)
	// MISSING: RdnSequence
	return out
}

func SubjectAltNames_ToProto(mapCtx *direct.MapContext, in *krm.SubjectAltNames) *pb.SubjectAltNames {
	if in == nil {
		return nil
	}
	out := &pb.SubjectAltNames{}
	out.DnsNames = in.DNSNames
	out.Uris = in.Uris
	out.EmailAddresses = in.EmailAddresses
	out.IpAddresses = in.IPAddresses
	out.CustomSans = direct.Slice_ToProto(mapCtx, in.CustomSans, CertificateAuthority_X509Extension_ToProto)
	return out
}

func CertificateAuthority_ObjectID_ToProto(mapCtx *direct.MapContext, in *krm.CertificateAuthority_ObjectID) *pb.ObjectId {
	if in == nil {
		return nil
	}
	out := &pb.ObjectId{}
	if path := in.ObjectIDPath; path != nil {
		out.ObjectIdPath = make([]int32, len(path))
		for i, v := range path {
			out.ObjectIdPath[i] = int32(v)
		}
	}
	return out
}

func CertificateAuthority_X509Extension_ToProto(mapCtx *direct.MapContext, in *krm.CertificateAuthority_X509Extension) *pb.X509Extension {
	if in == nil {
		return nil
	}
	out := &pb.X509Extension{}
	out.ObjectId = CertificateAuthority_ObjectID_ToProto(mapCtx, &in.ObjectID)
	out.Critical = direct.ValueOf(in.Critical)
	out.Value = []byte(in.Value)
	return out
}

func CertificateAuthority_KeyUsage_ToProto(mapCtx *direct.MapContext, in *krm.CertificateAuthority_KeyUsage) *pb.KeyUsage {
	if in == nil {
		return nil
	}
	out := &pb.KeyUsage{}
	out.BaseKeyUsage = KeyUsage_KeyUsageOptions_ToProto(mapCtx, in.BaseKeyUsage)
	out.ExtendedKeyUsage = KeyUsage_ExtendedKeyUsageOptions_ToProto(mapCtx, in.ExtendedKeyUsage)
	out.UnknownExtendedKeyUsages = direct.Slice_ToProto(mapCtx, in.UnknownExtendedKeyUsages, CertificateAuthority_ObjectID_ToProto)
	return out
}

func KeyUsage_KeyUsageOptions_ToProto(mapCtx *direct.MapContext, in *krm.KeyUsage_KeyUsageOptions) *pb.KeyUsage_KeyUsageOptions {
	if in == nil {
		return nil
	}
	out := &pb.KeyUsage_KeyUsageOptions{}
	out.DigitalSignature = direct.ValueOf(in.DigitalSignature)
	out.ContentCommitment = direct.ValueOf(in.ContentCommitment)
	out.KeyEncipherment = direct.ValueOf(in.KeyEncipherment)
	out.DataEncipherment = direct.ValueOf(in.DataEncipherment)
	out.KeyAgreement = direct.ValueOf(in.KeyAgreement)
	out.CertSign = direct.ValueOf(in.CertSign)
	out.CrlSign = direct.ValueOf(in.CrlSign)
	out.EncipherOnly = direct.ValueOf(in.EncipherOnly)
	out.DecipherOnly = direct.ValueOf(in.DecipherOnly)
	return out
}

func KeyUsage_ExtendedKeyUsageOptions_ToProto(mapCtx *direct.MapContext, in *krm.KeyUsage_ExtendedKeyUsageOptions) *pb.KeyUsage_ExtendedKeyUsageOptions {
	if in == nil {
		return nil
	}
	out := &pb.KeyUsage_ExtendedKeyUsageOptions{}
	out.ServerAuth = direct.ValueOf(in.ServerAuth)
	out.ClientAuth = direct.ValueOf(in.ClientAuth)
	out.CodeSigning = direct.ValueOf(in.CodeSigning)
	out.EmailProtection = direct.ValueOf(in.EmailProtection)
	out.TimeStamping = direct.ValueOf(in.TimeStamping)
	out.OcspSigning = direct.ValueOf(in.OcspSigning)
	return out
}

func X509Parameters_CAOptions_ToProto(mapCtx *direct.MapContext, in *krm.X509Parameters_CAOptions) *pb.X509Parameters_CaOptions {
	if in == nil {
		return nil
	}
	out := &pb.X509Parameters_CaOptions{}
	out.IsCa = in.IsCA
	out.MaxIssuerPathLength = in.MaxIssuerPathLength
	return out
}

func CertificateConfig_X509Config_ToProto(mapCtx *direct.MapContext, in *krm.CertificateConfig_X509Config) *pb.X509Parameters {
	if in == nil {
		return nil
	}
	out := &pb.X509Parameters{}
	out.AdditionalExtensions = direct.Slice_ToProto(mapCtx, in.AdditionalExtensions, CertificateAuthority_X509Extension_ToProto)
	out.CaOptions = X509Parameters_CAOptions_ToProto(mapCtx, in.CaOptions)
	out.KeyUsage = CertificateAuthority_KeyUsage_ToProto(mapCtx, in.KeyUsage)
	out.PolicyIds = direct.Slice_ToProto(mapCtx, in.PolicyIds, CertificateAuthority_ObjectID_ToProto)
	out.AiaOcspServers = in.AiaOcspServers
	out.NameConstraints = nameConstraintsToProto(in.NameConstraints)
	return out
}

func CertificateAuthority_KeyVersionSpec_ToProto(mapCtx *direct.MapContext, in *krm.CertificateAuthority_KeyVersionSpec) *pb.CertificateAuthority_KeyVersionSpec {
	if in == nil {
		return nil
	}
	out := &pb.CertificateAuthority_KeyVersionSpec{}
	if in.Algorithm != nil {
		out.KeyVersion = &pb.CertificateAuthority_KeyVersionSpec_Algorithm{
			Algorithm: direct.Enum_ToProto[pb.CertificateAuthority_SignHashAlgorithm](mapCtx, in.Algorithm),
		}
	}
	if in.CloudKmsKeyVersionRef != nil {
		out.KeyVersion = &pb.CertificateAuthority_KeyVersionSpec_CloudKmsKeyVersion{
			CloudKmsKeyVersion: in.CloudKmsKeyVersionRef.External,
		}
	}
	return out
}

func CertificateAuthority_UserDefinedAccessUrls_ToProto(mapCtx *direct.MapContext, in *krm.CertificateAuthority_UserDefinedAccessUrls) *pb.CertificateAuthority_UserDefinedAccessUrls {
	if in == nil {
		return nil
	}
	out := &pb.CertificateAuthority_UserDefinedAccessUrls{}
	out.AiaIssuingCertificateUrls = in.AiaIssuingCertificateUrls
	out.CrlAccessUrls = in.CrlAccessUrls
	return out
}
