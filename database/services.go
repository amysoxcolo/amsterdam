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
	"slices"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"gopkg.in/yaml.v3"
)

// ServiceVTable is a serioes of functions called for services on specific events.
type ServiceVTable interface {
	OnNewCommunity(*Community) error
	OnDeleteCommunity(int32) error
	OnUserJoinCommunity(*Community, *User) error
	OnUserLeaveCommunity(*Community, *User) error
}

// emptyServiceVTable is a default ServiceVTable that does nothing.
type emptyServiceVTable struct{}

func (*emptyServiceVTable) OnNewCommunity(*Community) error {
	return nil
}

func (*emptyServiceVTable) OnDeleteCommunity(int32) error {
	return nil
}

func (*emptyServiceVTable) OnUserJoinCommunity(*Community, *User) error {
	return nil
}

func (*emptyServiceVTable) OnUserLeaveCommunity(*Community, *User) error {
	return nil
}

// ServiceDef holds the definition for an individual service.
type ServiceDef struct {
	Id                string `yaml:"id"`
	Index             int16  `yaml:"index"`
	Default           bool   `yaml:"default"`
	Locked            bool   `yaml:"locked"`
	RequirePermission string `yaml:"requirePermission"`
	RequireRole       string `yaml:"requireRole"`
	LinkSequence      int    `yaml:"linkSequence"`
	Link              string `yaml:"link"`
	Title             string `yaml:"title"`
	vtable            ServiceVTable
}

// ServiceDomain holds each individual configured service domain.
type ServiceDomain struct {
	DomainName string       `yaml:"domain"`
	Services   []ServiceDef `yaml:"services"`
	byId       map[string]*ServiceDef
	byIndex    map[int16]*ServiceDef
	seqOrder   []*ServiceDef
}

// ServiceConfiguration holds the service configuration.
type ServiceConfiguration struct {
	Domains []ServiceDomain `yaml:"domains"`
	byName  map[string]*ServiceDomain
}

//go:embed servicedefs.yaml
var initServiceData []byte

// The service configuration loaded from YAML.
var serviceRoot ServiceConfiguration

// The services cache for communities.
var servicesCache *lru.TwoQueueCache

// Mutex on the services cache.
var servicesCacheMutex sync.Mutex

// init loads the service configuration and builds all the internal indexes.
func init() {
	var err error
	if err = yaml.Unmarshal(initServiceData, &serviceRoot); err != nil {
		panic(err) // can't happen
	}
	serviceRoot.byName = make(map[string]*ServiceDomain)
	for i, dom := range serviceRoot.Domains {
		serviceRoot.Domains[i].byId = make(map[string]*ServiceDef)
		serviceRoot.Domains[i].byIndex = make(map[int16]*ServiceDef)
		sqo := make([]*ServiceDef, 0, len(serviceRoot.Domains[i].Services))
		for j, svc := range serviceRoot.Domains[i].Services {
			serviceRoot.Domains[i].byId[svc.Id] = &(serviceRoot.Domains[i].Services[j])
			serviceRoot.Domains[i].byIndex[svc.Index] = &(serviceRoot.Domains[i].Services[j])
			sqo = append(sqo, &(serviceRoot.Domains[i].Services[j]))
		}
		slices.SortFunc(sqo, func(a, b *ServiceDef) int {
			return a.LinkSequence - b.LinkSequence
		})
		serviceRoot.Domains[i].seqOrder = sqo
		serviceRoot.byName[dom.DomainName] = &(serviceRoot.Domains[i])
	}
	dom := serviceRoot.byName["community"]
	empty := emptyServiceVTable{}
	dom.byId["Profile"].vtable = &empty
	dom.byId["Admin"].vtable = &empty
	dom.byId["SysAdmin"].vtable = &empty
	dom.byId["Conference"].vtable = &empty // TODO
	dom.byId["Members"].vtable = &empty
	servicesCache, err = lru.New2Q(50)
	if err != nil {
		panic(err)
	}
}

/* AmGetCommunityServices returns all the community service definitions for a community.
 * Parameters:
 *     cid - Community ID to get services for.
 * Returns:
 *     Array of ServiceDef pointers for the community's services.
 *     Standard Go error status.
 */
func AmGetCommunityServices(cid int32) ([]*ServiceDef, error) {
	servicesCacheMutex.Lock()
	defer servicesCacheMutex.Unlock()
	rc, ok := servicesCache.Get(cid)
	if !ok {
		rs, err := amdb.Query("SELECT ftr_code FROM commftrs WHERE commid = ?", cid)
		if err != nil {
			return nil, err
		}
		dom := serviceRoot.byName["community"]
		a := make([]*ServiceDef, 0, len(dom.Services))
		for rs.Next() {
			var ndx int16
			rs.Scan(&ndx)
			a = append(a, dom.byIndex[ndx])
		}
		servicesCache.Add(cid, a)
		rc = a
	}
	return rc.([]*ServiceDef), nil
}

/* AmEstablishCommunityServices establishes the service (feature) records for a new community,
 * and allows the services to establish themselves.
 * Parameters:
 *     c - The new community.
 * Returns:
 *     Standard Go error status.
 */
func AmEstablishCommunityServices(c *Community) error {
	dom := serviceRoot.byName["community"]
	a := make([]*ServiceDef, 0, len(dom.Services))
	for i, svc := range dom.Services {
		if svc.Default {
			_, err := amdb.Exec("INSERT INTO commftrs (commid, ftr_code) VALUES (?, ?)", c.Id, svc.Index)
			if err != nil {
				return err
			}
			a = append(a, &(dom.Services[i]))
		}
	}
	servicesCacheMutex.Lock()
	servicesCache.Add(c.Id, a)
	servicesCacheMutex.Unlock()
	for _, svc := range a {
		err := svc.vtable.OnNewCommunity(c)
		if err != nil {
			return err
		}
	}
	return nil
}

/* AmDeleteCommunityServices cleans up all services associated with a community that has gone away,
 * and then cleans up the service records.
 * Parameters:
 *     cid - The ID of the departing community.
 * Returns:
 *     Standard Go error status.
 */
func AmDeleteCommunityServices(cid int32) error {
	arr, err := AmGetCommunityServices(cid)
	if err == nil {
		for _, svc := range arr {
			err = svc.vtable.OnDeleteCommunity(cid)
			if err != nil {
				break
			}
		}
	}
	if err == nil {
		_, err = amdb.Exec("DELETE FROM commftrs WHERE commid = ?", cid)
		servicesCacheMutex.Lock()
		servicesCache.Remove(cid)
		servicesCacheMutex.Unlock()
	}
	return err
}

/* AmOnUserJoinCommunityServices gives services a chance to update themselves when a user joins a community.
 * Parameters:
 *     c - The community that is being joined.
 *     u - The user leaving that community.
 * Returns:
 *     Standard Go error status.
 */
func AmOnUserJoinCommunityServices(c *Community, u *User) error {
	arr, err := AmGetCommunityServices(c.Id)
	if err == nil {
		for _, svc := range arr {
			err = svc.vtable.OnUserJoinCommunity(c, u)
			if err != nil {
				break
			}
		}
	}
	return err
}

/* AmOnUserLeaveCommunityServices gives services a chance to update themselves when a user leaves a community.
 * Parameters:
 *     c - The community that is being left.
 *     u - The user leaving that community.
 * Returns:
 *     Standard Go error status.
 */
func AmOnUserLeaveCommunityServices(c *Community, u *User) error {
	arr, err := AmGetCommunityServices(c.Id)
	if err == nil {
		for _, svc := range arr {
			err = svc.vtable.OnUserLeaveCommunity(c, u)
			if err != nil {
				break
			}
		}
	}
	return err
}
