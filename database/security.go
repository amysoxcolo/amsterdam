/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The database package contains database management and storage logic.
package database

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

/* securityScope defines nested "scopes" within the security level system.  Each scope is numerically nested
 * inside the previous one and outside the next one.  Each scope has a "low band" of values (for ordinary users)
 * and a "high band" of values (for users with administrative privilege).
 */
type securityScope struct {
	lowbandLow   uint16
	lowbandHigh  uint16
	highbandLow  uint16
	highbandHigh uint16
}

// scopelist defines the boundaries of the known scopes.  There are 16 in all, most are unused.
var scopelist = []securityScope{
	{0, 1999, 63000, 64999}, // global scope
	{2000, 3999, 61000, 62999},
	{4000, 5999, 59000, 60999},
	{6000, 7999, 57000, 58999}, // community scope
	{8000, 9999, 55000, 56999},
	{10000, 11999, 53000, 54999},
	{12000, 13999, 51000, 52999}, // conference scope
	{14000, 15999, 49000, 50999},
	{16000, 17999, 47000, 48999},
	{18000, 19999, 45000, 46999},
	{20000, 21999, 43000, 44999},
	{22000, 23999, 41000, 42999},
	{24000, 25999, 39000, 40999},
	{26000, 27999, 37000, 38999},
	{28000, 29999, 35000, 36999},
	{30000, 31999, 33000, 34999},
}

// CfgScope is a configured scope.
type CfgScope struct {
	Name   string `yaml:"name"`
	Index  int    `yaml:"index"`
	bounds *securityScope
}

// CfgRole is a configured role.
type CfgRole struct {
	Internal string `yaml:"name"`
	Display  string `yaml:"display"`
	Scope    string `yaml:"scope"`
	Value    string `yaml:"value"`
	level    uint16
}

// CfgDefault is a configured default value.
type CfgDefault struct {
	Name    string `yaml:"name"`
	Role    string `yaml:"role"`
	roleptr *CfgRole
}

// CfgRoleList is a configured role list.
type CfgRoleList struct {
	Name     string   `yaml:"name"`
	DDefault string   `yaml:"default"`
	DRoles   []string `yaml:"roles"`
	defptr   *CfgRole
	roleptrs []*CfgRole
}

// CfgPermission is a configured permission.
type CfgPermission struct {
	Name  string `yaml:"name"`
	Role  string `yaml:"role"`
	level uint16
}

// CfgSecurityDefs is the master structure for security definitions.
type CfgSecurityDefs struct {
	Scopes      []CfgScope      `yaml:"scopes"`
	Roles       []CfgRole       `yaml:"roles"`
	Defaults    []CfgDefault    `yaml:"defaults"`
	Lists       []CfgRoleList   `yaml:"lists"`
	Permissions []CfgPermission `yaml:"permissions"`
	scopeMap    map[string]*CfgScope
	roleMap     map[string]*CfgRole
	defaultsMap map[string]*CfgDefault
	listsMap    map[string]*CfgRoleList
	permsMap    map[string]*CfgPermission
}

//go:embed securitydefs.yaml
var initSecurityData []byte

// securityRoot contains the root-level security information.
var securityRoot CfgSecurityDefs

// parseLevelValue is a helper which parses the role level definition strings.
func parseLevelValue(scope *securityScope, value string) uint16 {
	switch value {
	case "LMIN":
		return scope.lowbandLow
	case "LMAX":
		return scope.lowbandHigh
	case "HMIN":
		return scope.highbandLow
	case "HMAX":
		return scope.highbandHigh
	}
	if strings.HasPrefix(value, "L+") {
		v, err := strconv.Atoi(value[2:])
		if err != nil {
			panic(err)
		}
		return scope.lowbandLow + uint16(v)
	}
	if strings.HasPrefix(value, "H+") {
		v, err := strconv.Atoi(value[2:])
		if err != nil {
			panic(err)
		}
		return scope.highbandLow + uint16(v)
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return uint16(v)
}

// init sets up all the security data.
func init() {
	if err := yaml.Unmarshal(initSecurityData, &securityRoot); err != nil {
		panic(err) // can't happen
	}
	securityRoot.scopeMap = make(map[string]*CfgScope)
	for i, sc := range securityRoot.Scopes {
		securityRoot.Scopes[i].bounds = &(scopelist[sc.Index])
		securityRoot.scopeMap[sc.Name] = &(securityRoot.Scopes[i])
	}
	securityRoot.roleMap = make(map[string]*CfgRole)
	for i, ro := range securityRoot.Roles {
		scope := securityRoot.scopeMap[ro.Scope]
		securityRoot.Roles[i].level = parseLevelValue(scope.bounds, ro.Value)
		securityRoot.roleMap[ro.Internal] = &(securityRoot.Roles[i])
	}
	securityRoot.defaultsMap = make(map[string]*CfgDefault)
	for i, def := range securityRoot.Defaults {
		securityRoot.Defaults[i].roleptr = securityRoot.roleMap[def.Role]
		securityRoot.defaultsMap[def.Name] = &(securityRoot.Defaults[i])
	}
	securityRoot.listsMap = make(map[string]*CfgRoleList)
	for i, li := range securityRoot.Lists {
		if li.DDefault != "" {
			securityRoot.Lists[i].defptr = securityRoot.roleMap[li.DDefault]
		}
		securityRoot.Lists[i].roleptrs = make([]*CfgRole, len(li.DRoles))
		for j, rn := range li.DRoles {
			securityRoot.Lists[i].roleptrs[j] = securityRoot.roleMap[rn]
		}
		securityRoot.listsMap[li.Name] = &(securityRoot.Lists[i])
	}
	securityRoot.permsMap = make(map[string]*CfgPermission)
	for i, pm := range securityRoot.Permissions {
		securityRoot.Permissions[i].level = securityRoot.roleMap[pm.Role].level
		securityRoot.permsMap[pm.Name] = &(securityRoot.Permissions[i])
	}
}

// Role defines a security role.
type Role interface {
	ID() string
	Name() string
	Level() uint16
	LevelStr() string
}

func (r *CfgRole) ID() string {
	return r.Internal
}

func (r *CfgRole) Name() string {
	return r.Display
}

func (r *CfgRole) Level() uint16 {
	return r.level
}

func (r *CfgRole) LevelStr() string {
	return fmt.Sprintf("%d", r.level)
}

// RoleList defines a list of security roles.
type RoleList interface {
	Roles() []Role
	Default() Role
	FindForLevel(uint16) Role
}

func (r *CfgRoleList) Roles() []Role {
	rc := make([]Role, len(r.roleptrs))
	for i := range r.roleptrs {
		rc[i] = r.roleptrs[i]
	}
	return rc
}

func (r *CfgRoleList) Default() Role {
	return r.defptr
}

func (r *CfgRoleList) FindForLevel(level uint16) Role {
	for _, rp := range r.roleptrs {
		if rp.level == level {
			return rp
		}
	}
	return nil
}

/* AmRole returns a Role given a string ID.
 * Parameters:
 *     id - ID of the role to look up.
 * Returns:
 *     The specified role.
 */
func AmRole(id string) Role {
	return securityRoot.roleMap[id]
}

/* AmDefaultRole returns a Role given a default ID.
 * Parameters:
 *     id - ID of the default to look up.
 * Returns:
 *     The specified role.
 */
func AmDefaultRole(id string) Role {
	return securityRoot.defaultsMap[id].roleptr
}

/* AmRoleList returns a RoleList given a list ID.
 * Parameters:
 *     id - ID of the list to look up.
 * Returns:
 *     The specified role list.
 */
func AmRoleList(id string) RoleList {
	return securityRoot.listsMap[id]
}

/* AmTestPermission tests a specified access level to see if it satisfies the given permission.
 * Parameters:
 *     id - ID of the permission to check.
 *     level - The access level to be verified.
 * Returns:
 *     true if the permission test is satisfied, false if not.
 */
func AmTestPermission(id string, level uint16) bool {
	return securityRoot.permsMap[id].level <= level
}

// AmPermissionLevel returns a level value for a permission.
func AmPermissionLevel(id string) uint16 {
	return securityRoot.permsMap[id].level
}

/* AmCombinePermissionRole combines a permission and a role into a single permission level.
 * Parameters:
 *     perm - Permission to use.
 *     role - Role to use.
 * Returns:
 *     The combined permission level.
 */
func AmCombinePermissionRole(perm string, role string) uint16 {
	p1 := securityRoot.permsMap[perm].level
	p2 := securityRoot.roleMap[role].level
	if p1 > p2 {
		return p1
	}
	return p2
}
