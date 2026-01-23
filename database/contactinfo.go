/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The database package contains database management and storage logic.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// ContactInfo stores the contact information for a user or community.
type ContactInfo struct {
	Mutex        sync.Mutex
	ContactId    int32      `db:"contactid"`
	GivenName    *string    `db:"given_name"`
	FamilyName   *string    `db:"family_name"`
	MiddleInit   *string    `db:"middle_init"`
	Prefix       *string    `db:"prefix"`
	Suffix       *string    `db:"suffix"`
	Company      *string    `db:"company"`
	Addr1        *string    `db:"addr1"`
	Addr2        *string    `db:"addr2"`
	Locality     *string    `db:"locality"`
	Region       *string    `db:"region"`
	PostalCode   *string    `db:"pcode"`
	Country      *string    `db:"country"`
	Phone        *string    `db:"phone"`
	Fax          *string    `db:"fax"`
	Mobile       *string    `db:"mobile"`
	Email        *string    `db:"email"`
	PrivateAddr  bool       `db:"pvt_addr"`
	PrivatePhone bool       `db:"pvt_phone"`
	PrivateFax   bool       `db:"pvt_fax"`
	PrivateEmail bool       `db:"pvt_email"`
	OwnerUid     int32      `db:"owner_uid"`
	OwnerCommId  int32      `db:"owner_commid"`
	PhotoURL     *string    `db:"photo_url"`
	URL          *string    `db:"url"`
	LastUpdate   *time.Time `db:"lastupdate"`
}

// lookupCommunityContact looks up the ID of a contact for a community.
func lookupCommunityContact(ctx context.Context, id int32) (int32, error) {
	var rc int32 = -1
	row := amdb.QueryRowContext(ctx, "SELECT contactid FROM contacts WHERE owner_commid = ?", id)
	err := row.Scan(&rc)
	return rc, err
}

// lookupUserContact looks up the ID of a contact for a user.
func lookupUserContact(ctx context.Context, uid int32) (int32, error) {
	var rc int32 = -1
	row := amdb.QueryRowContext(ctx, "SELECT contactid FROM contacts WHERE owner_uid = ? AND owner_commid = -1", uid)
	err := row.Scan(&rc)
	return rc, err
}

// FullName returns the full name inside this contact info.
func (ci *ContactInfo) FullName(ps bool) string {
	var b strings.Builder
	writeSpace := false
	if ps && ci.Prefix != nil && strings.TrimSpace(*ci.Prefix) != "" {
		b.WriteString(*ci.Prefix)
		writeSpace = true
	}
	if ci.GivenName != nil && strings.TrimSpace(*ci.GivenName) != "" {
		if writeSpace {
			b.WriteString(" ")
		}
		b.WriteString(*ci.GivenName)
		writeSpace = true
	}
	if ci.MiddleInit != nil && strings.TrimSpace(*ci.MiddleInit) != "" {
		if writeSpace {
			b.WriteString(" ")
		}
		b.WriteString(*ci.MiddleInit)
		b.WriteString(".")
		writeSpace = true
	}
	if ci.FamilyName != nil && strings.TrimSpace(*ci.FamilyName) != "" {
		if writeSpace {
			b.WriteString(" ")
		}
		b.WriteString(*ci.FamilyName)
		writeSpace = true
	}
	if ps && ci.Suffix != nil && strings.TrimSpace(*ci.Suffix) != "" {
		if writeSpace {
			b.WriteString(" ")
		}
		b.WriteString(*ci.Suffix)
	}
	return b.String()
}

/* Save saves the contact info to the database.
 * Parameters:
 *     ctx - Standard Go context value.
 * Returns:
 *     true if the E-mail address on this account has been changed, false if not.
 *     Standard Go error status.
 */
func (ci *ContactInfo) Save(ctx context.Context) (bool, error) {
	ci.Mutex.Lock()
	defer ci.Mutex.Unlock()

	// Determine whether we're doing an UPDATE or an INSERT, and whether the E-mail address is changing.
	updateMode := false
	emailChange := true
	if ci.ContactId <= 0 {
		var nx int32
		var err error
		if ci.OwnerCommId > 0 {
			nx, err = lookupCommunityContact(ctx, ci.OwnerCommId)
		} else {
			nx, err = lookupUserContact(ctx, ci.OwnerUid)
		}
		if err != nil {
			return false, err
		}
		if nx > 0 {
			ci.ContactId = nx
			updateMode = true
			emailChange = false
		}
	} else {
		updateMode = true
		emailChange = false
	}
	if !emailChange {
		// we don't THINK the E-mail address is changing, but we could be wrong...
		row := amdb.QueryRowContext(ctx, "SELECT contactid FROM contacts WHERE contactid = ? AND email = ?", ci.ContactId, ci.Email)
		err := row.Err()
		if err == sql.ErrNoRows {
			emailChange = true
		} else if err != nil {
			return false, err
		}
	}
	// Handle the database heavy lifting.
	if updateMode {
		_, err := amdb.NamedExecContext(ctx, `UPDATE contacts SET given_name = :given_name, family_name = :family_name, middle_init = :middle_init,
		    prefix = :prefix, suffix = :suffix, company = :company, addr1 = :addr1, addr2 = :addr2, locality = :locality, region = :region,
			pcode = :pcode, country = :country, phone = :phone, fax = :fax, mobile = :mobile, email = :email, pvt_addr = :pvt_addr,
			pvt_phone = :pvt_phone, pvt_fax = :pvt_fax, pvt_email = :pvt_email, photo_url = :photo_url, url = :url, lastupdate = NOW()
			WHERE contactid = :contactid`, ci)
		if err != nil {
			return false, err
		}
		contactCache.Add(ci.ContactId, ci)
	} else {
		res, err := amdb.NamedExecContext(ctx, `INSERT INTO contacts (given_name, family_name, middle_init, prefix, suffix, company, addr1,
        	addr2, locality, region, pcode, country, phone, fax, mobile, email, pvt_addr, pvt_phone, pvt_fax,
    		pvt_email, owner_uid, owner_commid, photo_url, url, lastupdate)
			VALUES (:given_name, :family_name, :middle_init, :prefix, :suffix, :company, :addr1, :addr2, :locality,
			:region, :pcode, :country, :phone, :fax, :mobile, :email, :pvt_addr, :pvt_phone, :pvt_fax, :pvt_email,
			:owner_uid, :owner_commid, :photo_url, :url, NOW())`, ci)
		if err != nil {
			return false, err
		}
		lii, _ := res.LastInsertId()
		ci.ContactId = int32(lii)
		contactCache.Add(ci.ContactId, ci)
	}
	// Refresh the last update date.
	row := amdb.QueryRowContext(ctx, "SELECT lastupdate FROM contacts WHERE contactid = ?", ci.ContactId)
	err := row.Scan(&(ci.LastUpdate))
	if err != nil {
		return false, err
	}
	return emailChange, err
}

// Clone makes a copy of the ContactInfo.
func (ci *ContactInfo) Clone() *ContactInfo {
	newstr := ContactInfo{
		ContactId:    ci.ContactId,
		GivenName:    ci.GivenName,
		FamilyName:   ci.FamilyName,
		MiddleInit:   ci.MiddleInit,
		Prefix:       ci.Prefix,
		Suffix:       ci.Suffix,
		Company:      ci.Company,
		Addr1:        ci.Addr1,
		Addr2:        ci.Addr2,
		Locality:     ci.Locality,
		Region:       ci.Region,
		PostalCode:   ci.PostalCode,
		Country:      ci.Country,
		Phone:        ci.Phone,
		Fax:          ci.Fax,
		Mobile:       ci.Mobile,
		Email:        ci.Mobile,
		PrivateAddr:  ci.PrivateAddr,
		PrivatePhone: ci.PrivatePhone,
		PrivateFax:   ci.PrivateFax,
		PrivateEmail: ci.PrivateEmail,
		OwnerUid:     ci.OwnerUid,
		OwnerCommId:  ci.OwnerCommId,
		PhotoURL:     ci.PhotoURL,
		URL:          ci.URL,
		LastUpdate:   ci.LastUpdate,
	}
	return &newstr
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
func internalContactInfo(ctx context.Context, id int32) (*ContactInfo, error) {
	var dbdata []ContactInfo
	err := amdb.SelectContext(ctx, &dbdata, "SELECT * from contacts WHERE contactid = ?", id)
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
 *     ctx - Standard Go context value.
 *     id - The contact info ID top retrieve.
 * Returns:
 *	   ContactInfo retrieved, or nil.
 *     Standard Go error status.
 */
func AmGetContactInfo(ctx context.Context, id int32) (*ContactInfo, error) {
	getContactMutex.Lock()
	defer getContactMutex.Unlock()
	rc, ok := contactCache.Get(id)
	if ok {
		return rc.(*ContactInfo), nil
	}
	rc2, err := internalContactInfo(ctx, id)
	if err == nil {
		if rc2 != nil {
			contactCache.Add(id, rc2)
		}
		return rc2, nil
	}
	return nil, err
}

/* AmNewUserContactInfo creates a new contact info record for the user.
 * Parameters:
 *     uid - The UID of the owner of this contact info.
 * Returns:
 *     New ContactInfo structure.
 */
func AmNewUserContactInfo(uid int32) *ContactInfo {
	rc := ContactInfo{OwnerUid: uid, OwnerCommId: -1}
	return &rc
}

/* AmNewCommunityContactInfo creates a new contact info record for the community.
 * Parameters:
 *     uid - The UID of the host of this community.
 *     cid - The community ID of the owning community.
 * Returns:
 *     New ContactInfo structure.
 */
func AmNewCommunityContactInfo(uid int32, cid int32) *ContactInfo {
	rc := ContactInfo{OwnerUid: uid, OwnerCommId: cid}
	return &rc
}
