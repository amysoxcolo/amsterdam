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
	"fmt"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	log "github.com/sirupsen/logrus"
)

// AuditRecord holds an audit record instance.
type AuditRecord struct {
	Record int64     `db:"record"`  // audit record ID
	OnDate time.Time `db:"on_date"` // timestamp of event
	Event  int32     `db:"event"`   // ID of the event
	Uid    int32     `db:"uid"`     // user that performed the event
	CommId int32     `db:"commid"`  // community associated with the event
	IP     *string   `db:"ip"`      // IP address associated with the event
	Data1  *string   `db:"data1"`   // first data parameter
	Data2  *string   `db:"data2"`   // second data parameter
	Data3  *string   `db:"data3"`   // third data parameter
	Data4  *string   `db:"data4"`   // fourth data parameter
}

// These are the audit record types.
const (
	AuditPublishToFrontPage         = 1
	AuditLoginOK                    = 101
	AuditLoginFail                  = 102
	AuditAccountCreated             = 103
	AuditVerifyEmailOK              = 104
	AuditVerifyEmailFail            = 105
	AuditSetUserContactInfo         = 106
	AuditResendEmailConfirm         = 107
	AuditChangePassword             = 108
	AuditAdminSetUserContactInfo    = 109
	AuditAdminChangeUserPassword    = 110
	AuditAdminChangeUserAccount     = 111
	AuditAdminSetAccountSecurity    = 112
	AuditAdminLockUnlockAccount     = 113
	AuditCommunityCreate            = 201
	AuditCommunitySetMembership     = 202
	AuditCommuntiyContactInfo       = 203
	AuditCommunityFeatureSet        = 204
	AuditCommunityName              = 205
	AuditCommunityAlias             = 206
	AuditCommunityCategory          = 207
	AuditCommunityHideInfo          = 208
	AuditCommunityMembersOnly       = 209
	AuditCommunityJoinKey           = 210
	AuditCommunitySecurity          = 211
	AuditCommunityDelete            = 212
	AuditConferenceCreate           = 301
	AuditConferenceSecurity         = 302
	AuditConferenceName             = 303
	AuditConferenceAlias            = 304
	AuditConferenceMembership       = 305
	AuditConferenceCreateTopic      = 306
	AuditConferenceDeleteTopic      = 307
	AuditConferenceFreezeTopic      = 308
	AuditConferenceArchiveTopic     = 309
	AuditConferencePostMessage      = 310
	AuditConferenceHideMessage      = 311
	AuditConferenceScribbleMessage  = 312
	AuditConferenceNukeMessage      = 313
	AuditConferenceUploadAttachment = 314
	AuditConferenceDelete           = 315
	AuditConferenceMoveMessage      = 316
	AuditConferenceStickyTopic      = 317
)

// auditWriteQueue is a channel to store audit records in the background.
var auditWriteQueue chan *AuditRecord

/* AmNewAudit creates a new audit record.
 * Parameters:
 *     rectype - Audit record type.
 *     uid - User ID of the user.
 *     ip - User's IP address.
 *     data - Argument data values for the audit record.
 * Returns:
 *     The audit record pointer.
 */
func AmNewAudit(rectype int32, uid int32, ip string, data ...string) *AuditRecord {
	rc := AuditRecord{Event: rectype, Uid: uid, CommId: 0}
	if len(ip) > 0 {
		rc.IP = &ip
	}
	if data != nil {
		l := len(data)
		if l > 0 {
			rc.Data1 = &(data[0])
		}
		if l > 1 {
			rc.Data2 = &(data[1])
		}
		if l > 2 {
			rc.Data3 = &(data[2])
		}
		if l > 3 {
			rc.Data4 = &(data[3])
		}
	}
	return &rc
}

// Store stores the audit record in the database.
func (ar *AuditRecord) Store(ctx context.Context) error {
	if ar.Record > 0 {
		return fmt.Errorf("audit record %d already stored", ar.Record)
	}
	moment := time.Now().UTC()
	rs, err := amdb.ExecContext(ctx, `INSERT INTO audit (on_date, event, uid, commid, ip, data1, data2, data3, data4)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`, moment, ar.Event, ar.Uid, ar.CommId, ar.IP,
		ar.Data1, ar.Data2, ar.Data3, ar.Data4)
	if err != nil {
		return err
	}
	ar.Record, _ = rs.LastInsertId()
	ar.OnDate = moment
	return nil
}

// auditWriter is the routine that stores audit records in trhe background.
func auditWriter(workChan chan *AuditRecord, doneChan chan bool) {
	for ar := range workChan {
		err := ar.Store(context.Background())
		if err != nil {
			log.Errorf("dropped audit record (%+v) on the floor: %v", *ar, err)
		}
	}
	doneChan <- true
}

// AmStoreAudit stores the audit record in the background.
func AmStoreAudit(rec *AuditRecord) {
	if rec != nil {
		auditWriteQueue <- rec
	}
}

// setupAuditWriter sets up the background audit writer.
func setupAuditWriter() func() {
	auditWriteQueue = make(chan *AuditRecord, config.GlobalConfig.Tuning.Queues.AuditWrites)
	doneChan := make(chan bool)
	go auditWriter(auditWriteQueue, doneChan)
	return func() {
		close(auditWriteQueue)
		<-doneChan
	}
}
