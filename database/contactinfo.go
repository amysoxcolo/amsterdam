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
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// ContactInfo stores the contact information for a user or community.
type ContactInfo struct {
	ContactId    int32   `db:"contactid"`
	GivenName    string  `db:"given_name"`
	FamilyName   string  `db:"family_name"`
	MiddleInit   string  `db:"middle_init"`
	Prefix       *string `db:"prefix"`
	Suffix       *string `db:"suffix"`
	Company      *string `db:"company"`
	Addr1        *string `db:"addr1"`
	Addr2        *string `db:"addr2"`
	Locality     *string `db:"locality"`
	Region       *string `db:"region"`
	PostalCode   *string `db:"pcode"`
	Country      *string `db:"country"`
	Phone        *string `db:"phone"`
	Fax          *string `db:"fax"`
	Mobile       *string `db:"mobile"`
	Email        *string `db:"email"`
	PrivateAddr  bool    `db:"pvt_addr"`
	PrivatePhone bool    `db:"pvt_phone"`
	PrivateFax   bool    `db:"pvt_fax"`
	PrivateEmail bool    `db:"pvt_email"`
	OwnerUid     int32   `db:"owner_uid"`
	OwnerCommId  int32   `db:"owner_commid"`
	PhotoURL     *string `db:"photo_url"`
	URL          *string `db:"url"`
	LastUpdate   *time.Time
}

// contactCache is the cache for ContactInfo objects.
var contactCache *lru.TwoQueueCache = nil

// getContactMutex is a mutex on AmGetContactInfo.
var getContactMutex sync.Mutex

// init initializes the contact info cache.
func init() {
	var err error
	contactCache, err = lru.New2Q(100)
	if err != nil {
		panic(err)
	}
}

// internalContactInfo retrieves the contact info from the database.
func internalContactInfo(id int32) (*ContactInfo, error) {
	var dbdata []ContactInfo
	err := amdb.Select(&dbdata, "SELECT * from contacts WHERE contactid = ?", id)
	if err == nil {
		if len(dbdata) > 1 {
			err = fmt.Errorf("internalContactInfo(%d): Too many responses (%d)", id, len(dbdata))
		} else if len(dbdata) == 0 {
			return nil, nil
		} else {
			return &(dbdata[0]), nil
		}
	}
	return nil, err
}

/* AmGetContactInfo retrieves the contact info for a given identifier.
 * Parameters:
 *     id - The contact info ID top retrieve.
 * Returns:
 *
 */
func AmGetContactInfo(id int32) (*ContactInfo, error) {
	getContactMutex.Lock()
	defer getContactMutex.Unlock()
	rc, ok := contactCache.Get(id)
	if ok {
		return rc.(*ContactInfo), nil
	}
	rc2, err := internalContactInfo(id)
	if err == nil {
		if rc2 != nil {
			contactCache.Add(id, rc2)
		}
		return rc2, nil
	}
	return nil, err
}
