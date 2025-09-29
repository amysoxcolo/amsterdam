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
	"slices"
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
	{0, 1999, 63000, 64999},    // global scope
	{2000, 3999, 61000, 62999}, // community scope
	{4000, 5999, 59000, 60999}, // conference scope
	{6000, 7999, 57000, 58999},
	{8000, 9999, 55000, 56999},
	{10000, 11999, 53000, 54999},
	{12000, 13999, 51000, 52999},
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

// unrestrictedUserLevel is a user level that is above all "low" bands but below all "high" bands.
const unrestrictedUserLevel uint16 = 32500

// noAccessLevel is the level used to specify "no access," as it's above the highest value in the topmost scope.
const noAccessLevel uint16 = 65500

// Role defines a security role.
type Role interface {
	ID() string
	Name() string
	Level() uint16
}

// amRole is the internal implementation of the Role.
type amRole struct {
	id    string
	name  string
	level uint16
}

// ID returns the string identifier of the role.
func (r *amRole) ID() string {
	return r.id
}

// Name returns the textual name of the role.
func (r *amRole) Name() string {
	return r.name
}

// Level returns the access level of the role.
func (r *amRole) Level() uint16 {
	return r.level
}

// RoleList defines a list of security roles.
type RoleList interface {
	Roles() []Role
	Default() Role
}

// amRoleList is the internal implementation of RoleList.
type amRoleList struct {
	roleList    []Role
	defaultRole Role
}

// Roles returns the list of roles for a given RoleList.
func (rl *amRoleList) Roles() []Role {
	return rl.roleList
}

// Default returns the default role in a given RoleList.
func (rl *amRoleList) Default() Role {
	return rl.defaultRole
}

// roles holds all the defined roles.
var roles map[string]Role

// roleDefaults assigns roles to symbolic default names.
var roleDefaults map[string]Role

// roleLists holds all the defined role lists.
var roleLists map[string]RoleList

// permissions assigns roles to specific named permissions.
var permissions map[string]Role

// defineRole defines a role and adds it to the roles map.
func defineRole(id string, name string, level uint16) Role {
	r := amRole{id: id, name: name, level: level}
	roles[id] = &r
	return &r
}

// defineRoleList defines a role list and adds it to the roleLists map.
func defineRoleList(id string, roles []Role, defaultRole Role) {
	slices.SortFunc(roles, func(a Role, b Role) int {
		leva := int(a.Level())
		levb := int(b.Level())
		return leva - levb
	})
	rl := amRoleList{roleList: roles, defaultRole: defaultRole}
	roleLists[id] = &rl
}

/* AmRole returns a Role given a string ID.
 * Parameters:
 *     id - ID of the role to look up.
 * Returns:
 *     The specified role.
 */
func AmRole(id string) Role {
	return roles[id]
}

/* AmDefaultRole returns a Role diven a default ID.
 * Parameters:
 *     id - ID of the default to look up.
 * Returns:
 *     The specified role.
 */
func AmDefaultRole(id string) Role {
	return roleDefaults[id]
}

/* AmTestPermission tests a specified access level to see if it satisfies the given permission.
 * Parameters:
 *     id - ID of the permission to check.
 *     level - The access level to be verified.
 * Returns:
 *     true if the permission test is satisfied, false if not.
 */
func AmTestPermission(id string, level uint16) bool {
	return permissions[id].Level() < level
}

// init initializes all the security data.
func init() {
	// Initialize the roles.
	roles = make(map[string]Role)
	not := defineRole("NotInList", "Not in List", 0)
	uu := defineRole("UnrestrictedUser", "Unrestricted User", unrestrictedUserLevel)
	none := defineRole("NoAccess", "No Access", noAccessLevel)
	g_anon := defineRole("Global.Anonymous", "Anonymous User", scopelist[0].lowbandLow+100)
	g_unverf := defineRole("Global.Unverified", "Unauthenticated User", scopelist[0].lowbandLow+500)
	g_normal := defineRole("Global.Normal", "Normal User", scopelist[0].lowbandLow+1000)
	g_anyadmin := defineRole("Global.AnyAdmin", "Any System Administrator", scopelist[0].highbandLow)
	g_PFY := defineRole("Global.PFY", "System Assistant Administrator", scopelist[0].highbandLow+1000)
	g_BOFH := defineRole("Global.BOFH", "Global System Administrator", scopelist[0].highbandHigh)
	com_member := defineRole("Community.Member", "Community Member", scopelist[1].lowbandLow+500)
	com_anyadmin := defineRole("Community.AnyAdmin", "Any Community Administrator", scopelist[1].highbandLow)
	com_cohost := defineRole("Community.Cohost", "Community Co-Host", scopelist[1].highbandLow+1000)
	com_host := defineRole("Community.Host", "Community Host", scopelist[1].highbandLow+1500)

	// Initialize the defaults list.
	roleDefaults = make(map[string]Role)
	roleDefaults["Global.NewUser"] = g_unverf
	roleDefaults["Global.AfterVerify"] = g_normal
	roleDefaults["Global.AfterEmailChange"] = g_unverf
	roleDefaults["Community.NewUser"] = com_member
	roleDefaults["Community.Creator"] = com_host

	// Initialize the roles lists.
	roleLists = make(map[string]RoleList)
	defineRoleList("Global.UserLevels", []Role{g_anon, g_unverf, g_normal, uu}, nil)
	defineRoleList("Global.UserLevelsPFY", []Role{g_anon, g_unverf, g_normal, uu, g_PFY}, nil)
	defineRoleList("Global.CreateCommunity", []Role{g_normal, uu, g_anyadmin, g_PFY, g_BOFH}, g_normal)
	defineRoleList("Community.Read", []Role{g_anon, g_unverf, g_normal, com_member, uu, com_anyadmin, com_cohost, com_host, g_anyadmin}, com_member)
	defineRoleList("Community.Write", []Role{com_anyadmin, com_cohost, com_host, g_anyadmin, g_PFY, g_BOFH}, com_cohost)
	defineRoleList("Community.Create", []Role{g_normal, com_member, uu, com_anyadmin, com_cohost, com_host, g_anyadmin}, com_cohost)
	defineRoleList("Community.Delete", []Role{com_anyadmin, com_cohost, com_host, g_anyadmin, g_PFY, g_BOFH, none}, com_host)
	defineRoleList("Community.Join", []Role{g_anon, g_unverf, g_normal}, g_normal)
	defineRoleList("Community.UserLevels", []Role{not, g_anon, g_unverf, g_normal, com_member, uu, com_cohost}, nil)

	// Initialize the permissions lists.
	permissions = make(map[string]Role)
	permissions["Global.ShowHiddenCategories"] = g_anyadmin
	permissions["Global.NoEmailVerify"] = g_anyadmin
	permissions["Global.SeeHiddenContactInfo"] = g_anyadmin
	permissions["Global.SearchHiddenCommunities"] = g_anyadmin
	permissions["Global.ShowHiddenCommunities"] = g_anyadmin
	permissions["Global.SearchHiddenCategories"] = g_anyadmin
	permissions["Global.SysAdminAccess"] = g_anyadmin
	permissions["Global.PublishFP"] = g_anyadmin
	permissions["Global.DesignatePFY"] = g_BOFH
	permissions["Community.ShowAdmin"] = com_anyadmin
	permissions["Community.NoJoinRequired"] = g_anyadmin
	permissions["Community.NoKeyRequired"] = g_anyadmin
	permissions["Community.ShowHiddenMembers"] = com_anyadmin
	permissions["Community.ShowHiddenObjects"] = com_anyadmin
	permissions["Community.MassMail"] = com_anyadmin
}
