// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package iampolicy reads and changes the IAM policy of a Google Cloud
// resource, for the kinds that bind members to roles (IAMBinding and
// the additive kind). Written here. The bindings are changed by Terraform's
// rules (google_*_iam_binding replaces one role's members; google_*_iam_member
// adds one), read, not copied. The policy is read and written whole with its
// etag, so audit configs and every other role are carried through unchanged.
package iampolicy

import (
	"slices"
	"sort"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/genproto/googleapis/type/expr"
)

// Condition is a binding's IAM condition. A binding is identified by its
// role and its condition's title and expression, as Terraform's
// iamBindingKey does (the description is carried but is not part of the
// identity here).
type Condition struct {
	Title       string
	Description string
	Expression  string
}

func (c *Condition) matches(e *expr.Expr) bool {
	if c == nil {
		return e == nil
	}
	return e != nil && e.GetTitle() == c.Title && e.GetExpression() == c.Expression
}

func (c *Condition) expr() *expr.Expr {
	if c == nil {
		return nil
	}
	return &expr.Expr{Title: c.Title, Description: c.Description, Expression: c.Expression}
}

// canonical is a member in the form the API compares: it lowercases the
// identity of email-like members, as the API returns them.
func canonical(member string) string {
	prefix, rest, ok := strings.Cut(member, ":")
	if !ok {
		return member
	}
	switch prefix {
	case "user", "group", "serviceAccount", "domain", "deleted":
		return prefix + ":" + strings.ToLower(rest)
	}
	return member
}

// set is a set of members by canonical form, remembering one spelling.
type set map[string]string

func newSet(members []string) set {
	s := set{}
	for _, m := range members {
		s[canonical(m)] = m
	}
	return s
}

func (s set) equal(o set) bool {
	if len(s) != len(o) {
		return false
	}
	for k := range s {
		if _, ok := o[k]; !ok {
			return false
		}
	}
	return true
}

// Members returns the members of the binding for the role and condition
// (all of them, if the policy has the pair twice), sorted.
func Members(p *iampb.Policy, role string, cond *Condition) (members []string, found bool) {
	for _, b := range p.GetBindings() {
		if b.GetRole() == role && cond.matches(b.GetCondition()) {
			found = true
			members = append(members, b.GetMembers()...)
		}
	}
	sort.Strings(members)
	return slices.Compact(members), found
}

// SetMembers makes the policy's binding for the role and condition have
// exactly these members (the authoritative form): other members of that
// binding are removed, other roles are untouched. No members removes the
// binding. It reports whether the policy changed.
func SetMembers(p *iampb.Policy, role string, cond *Condition, members []string) bool {
	if len(members) == 0 {
		return Remove(p, role, cond)
	}
	desired := newSet(members)
	var have set
	var matching []int
	for i, b := range p.GetBindings() {
		if b.GetRole() == role && cond.matches(b.GetCondition()) {
			matching = append(matching, i)
			if have == nil {
				have = set{}
			}
			for k, v := range newSet(b.GetMembers()) {
				have[k] = v
			}
		}
	}
	if len(matching) == 1 && have.equal(desired) &&
		(cond == nil || p.Bindings[matching[0]].GetCondition().GetDescription() == cond.Description) {
		return false
	}
	nb := &iampb.Binding{Role: role, Members: sortedValues(desired), Condition: cond.expr()}
	if len(matching) == 0 {
		p.Bindings = append(p.Bindings, nb)
	} else {
		p.Bindings[matching[0]] = nb
		p.Bindings = removeIndexes(p.Bindings, matching[1:])
	}
	needVersion(p)
	return true
}

// AddMembers makes the policy's binding for the role and condition include
// these members (the additive form), leaving its other members alone. It
// reports whether the policy changed.
func AddMembers(p *iampb.Policy, role string, cond *Condition, members []string) bool {
	have, _ := Members(p, role, cond)
	merged := newSet(have)
	changed := false
	for _, m := range members {
		if _, ok := merged[canonical(m)]; !ok {
			merged[canonical(m)] = m
			changed = true
		}
	}
	if !changed {
		return false
	}
	return SetMembers(p, role, cond, sortedValues(merged))
}

// RemoveMembers takes these members out of the binding for the role and
// condition, and the binding itself if none are left. It reports whether
// the policy changed.
func RemoveMembers(p *iampb.Policy, role string, cond *Condition, members []string) bool {
	have, found := Members(p, role, cond)
	if !found {
		return false
	}
	left := newSet(have)
	changed := false
	for _, m := range members {
		if _, ok := left[canonical(m)]; ok {
			delete(left, canonical(m))
			changed = true
		}
	}
	if !changed {
		return false
	}
	return SetMembers(p, role, cond, sortedValues(left))
}

// Remove deletes the binding for the role and condition. It reports
// whether the policy changed.
func Remove(p *iampb.Policy, role string, cond *Condition) bool {
	var matching []int
	for i, b := range p.GetBindings() {
		if b.GetRole() == role && cond.matches(b.GetCondition()) {
			matching = append(matching, i)
		}
	}
	if len(matching) == 0 {
		return false
	}
	p.Bindings = removeIndexes(p.Bindings, matching)
	return true
}

// needVersion raises the policy to version 3 when a binding has a
// condition, as the API requires to keep them.
func needVersion(p *iampb.Policy) {
	for _, b := range p.GetBindings() {
		if b.GetCondition() != nil && p.Version < 3 {
			p.Version = 3
			return
		}
	}
}

func sortedValues(s set) []string {
	out := make([]string, 0, len(s))
	for _, v := range s {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func removeIndexes(bs []*iampb.Binding, idx []int) []*iampb.Binding {
	out := bs[:0:0]
	for i, b := range bs {
		if !slices.Contains(idx, i) {
			out = append(out, b)
		}
	}
	return out
}
