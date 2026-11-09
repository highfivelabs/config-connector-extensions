// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here, in the shape of Config Connector's identities.

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
)

var _ identity.Identity = &IAMMemberIdentity{}

// IAMMemberIdentity is a member of a role on a resource:
// {resource}#{role}#{member}, or {resource}#{role}#{member}#{condition
// title}, where the resource is its full name in the API and the member is
// as the API writes it (it has a colon, never a hash).
type IAMMemberIdentity struct {
	Resource, Role, Member, ConditionTitle string
}

func (i *IAMMemberIdentity) String() string {
	s := i.Resource + "#" + i.Role + "#" + i.Member
	if i.ConditionTitle != "" {
		s += "#" + i.ConditionTitle
	}
	return s
}

func (i *IAMMemberIdentity) FromExternal(ref string) error {
	t := strings.SplitN(ref, "#", 4)
	if len(t) < 3 || t[0] == "" || t[1] == "" || t[2] == "" {
		return fmt.Errorf("format of IAMMember external=%q was not known (use {resource}#{role}#{member} or {resource}#{role}#{member}#{condition title})", ref)
	}
	*i = IAMMemberIdentity{Resource: t[0], Role: t[1], Member: t[2]}
	if len(t) == 4 {
		i.ConditionTitle = t[3]
	}
	return nil
}
