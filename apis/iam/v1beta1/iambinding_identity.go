// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// Written here, in the shape of Config Connector's identities.

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
)

var _ identity.Identity = &IAMBindingIdentity{}

// IAMBindingIdentity is a binding on a resource:
// {resource}#{role}, or {resource}#{role}#{condition title}, where the
// resource is its full name in the API (projects/{project}/secrets/{id}).
// It does not name the kind of the resource: the name pattern says it.
type IAMBindingIdentity struct {
	Resource, Role, ConditionTitle string
}

func (i *IAMBindingIdentity) String() string {
	s := i.Resource + "#" + i.Role
	if i.ConditionTitle != "" {
		s += "#" + i.ConditionTitle
	}
	return s
}

func (i *IAMBindingIdentity) FromExternal(ref string) error {
	t := strings.SplitN(ref, "#", 3)
	if len(t) < 2 || t[0] == "" || t[1] == "" {
		return fmt.Errorf("format of IAMBinding external=%q was not known (use {resource}#{role} or {resource}#{role}#{condition title})", ref)
	}
	*i = IAMBindingIdentity{Resource: t[0], Role: t[1]}
	if len(t) == 3 {
		i.ConditionTitle = t[2]
	}
	return nil
}
