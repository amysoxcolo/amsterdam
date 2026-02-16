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
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	log "github.com/sirupsen/logrus"
)

// PostHeader represents the "header" of a post, everything except for its text and attachment.
type PostHeader struct {
	PostId       int64      `db:"postid"`        // ID of the post
	Parent       int64      `db:"parent"`        // ID of parent (unused?)
	TopicId      int32      `db:"topicid"`       // topic containing the post
	Num          int32      `db:"num"`           // post number
	LineCount    *int32     `db:"linecount"`     // number of lines
	CreatorUid   int32      `db:"creator_uid"`   // UID creating post
	Posted       time.Time  `db:"posted"`        // date posted
	Hidden       bool       `db:"hidden"`        // is post hidden?
	ScribbleUid  *int32     `db:"scribble_uid"`  // UID of who scribbled it
	ScribbleDate *time.Time `db:"scribble_date"` // when was it scribbled?
	Pseud        *string    `db:"pseud"`         // post's "pseud" (name/header)
}

type PostData struct {
	PostId int64   `db:"postid"` // ID of the post
	Data   *string `db:"data"`   // actual post data
}

// PostAttachInfo contains information about a file attachment to a post.
type PostAttachInfo struct {
	Filename string // name of attached file
	MIMEType string // MIME type of attached file
	Length   int32  // length in bytes of attached file
}

const (
	stgMethodPlain = 0 // attachment stored as raw data
	stgMethodGZIP  = 1 // attachment stored as GZIP data
)

// ErrNoPostData is returned if post data is missing.
var ErrNoPostData = errors.New("no post data")

// Creator returns the creator of the post.
func (p *PostHeader) Creator(ctx context.Context) (*User, error) {
	return AmGetUser(ctx, p.CreatorUid)
}

// IsScribbled returns true if the post has been scribbled, false if not.
func (p *PostHeader) IsScribbled() bool {
	return p.ScribbleUid != nil && p.ScribbleDate != nil
}

// IsPublished returns true if the post has been published to the front page.
func (p *PostHeader) IsPublished(ctx context.Context) (bool, error) {
	row := amdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM postpublish WHERE postid = ?", p.PostId)
	ct := 0
	err := row.Scan(&ct)
	return ct > 0, err
}

/* AttachmentInfo returns attachment information for a post.
 * Parameters:
 *     ctx - Standard Go context value.
 * Returns:
 *     Pointer to structure with post attachment info, or nil if there is no attachment.
 *     Standard Go error status.
 */
func (p *PostHeader) AttachmentInfo(ctx context.Context) (*PostAttachInfo, error) {
	if p.ScribbleDate != nil && p.ScribbleUid != nil {
		return nil, errors.New("no attachment data for scribbled post")
	}
	row := amdb.QueryRowContext(ctx, "SELECT filename, mimetype, datalen FROM postattach WHERE postid = ?", p.PostId)
	var rc PostAttachInfo
	err := row.Scan(&(rc.Filename), &(rc.MIMEType), &(rc.Length))
	switch err {
	case nil:
		return &rc, nil
	case sql.ErrNoRows:
		return nil, nil
	}
	return nil, err
}

/* AttachmentData returns attachment data for a post.
 * Parameters:
 *     ctx - Standard Go context value.
 *     bugWorkaround - Work around certain bugs in extracting compressed data, if true.
 * Returns:
 *     Attachment data as a byte array.
 *     Standard Go error status.
 */
func (p *PostHeader) AttachmentData(ctx context.Context, bugWorkaround bool) ([]byte, error) {
	if p.ScribbleDate != nil && p.ScribbleUid != nil {
		return nil, errors.New("no attachment data for scribbled post")
	}
	row := amdb.QueryRowContext(ctx, "SELECT datalen, stgmethod, data FROM postattach WHERE postid = ?", p.PostId)
	var datalen int32
	var stgmethod int16
	var dbdata []byte
	err := row.Scan(&datalen, &stgmethod, &dbdata)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if stgmethod == stgMethodPlain {
		return dbdata, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(dbdata))
	if err != nil {
		return nil, err
	}
	outdata := make([]byte, datalen)
	n, err := r.Read(outdata)
	r.Close()
	if err == io.EOF && n == int(datalen) {
		err = nil // we got everything, this isn't an error
	}
	if err != nil || n < int(datalen) {
		if err == nil {
			if bugWorkaround {
				log.Warnf("PostHeader.AttachmentData: bugged attachment on post #%d (expected %d bytes, got %d), truncating for retrieval", p.PostId, datalen, n)
				outdata = outdata[:n]
			} else {
				log.Errorf("PostHeader.AttachmentData: unable to read entire attachment to post #%d (expected %d bytes, got %d)", p.PostId, datalen, n)
				err = errors.New("unable to read entire attachment")
			}
		} else {
			log.Errorf("PostHeader.AttachmentData: error (%v) reading attachment to post #%d (expected %d bytes, got %d)", err, p.PostId, datalen, n)
		}
		return nil, err
	}
	return outdata, nil
}

/* SetAttachment sets the attachment data for a post.
 * Parameters:
 *     ctx - Standard Go context value.
 *     u - user attempting to upload attachment data
 *     fileName - Name of the original attachment file.
 *     mimeType - MIME type of the attachment data.
 *     length - Length of the attachment data in bytes.
 *     data - The attachment data itself.
 * Returns:
 *     Standard Go error status.
 */
func (p *PostHeader) SetAttachment(ctx context.Context, u *User, fileName string, mimeType string, length int32, data []byte, ipaddr string) error {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	if p.ScribbleDate != nil && p.ScribbleUid != nil {
		return errors.New("cannot attach to scribbled post")
	}
	if u.Uid != p.CreatorUid {
		return errors.New("cannot attach to a post that is not yours")
	}
	ai, err := p.AttachmentInfo(ctx)
	if err != nil {
		return err
	}
	if ai != nil {
		return errors.New("attachment already present for this post")
	}
	if length > config.GlobalComputedConfig.UploadMaxSize {
		return fmt.Errorf("file too large to be attached; maximum size is %s", config.GlobalConfig.Posting.Uploads.MaxSize)
	}

	// Compress the data with GZIP if we need to.
	var stgmethod int16
	var realData []byte
	if _, ok := config.GlobalComputedConfig.UploadNoCompress[mimeType]; ok {
		realData = data
		stgmethod = stgMethodPlain
	} else {
		buf := new(bytes.Buffer)
		w := gzip.NewWriter(buf)
		_, err := w.Write(data)
		if err == nil {
			err = w.Close()
		}
		if err != nil {
			return err
		}
		realData = buf.Bytes()
		stgmethod = stgMethodGZIP
	}

	// Write to the database.
	_, err = amdb.ExecContext(ctx, "INSERT INTO postattach (postid, datalen, filename, mimetype, stgmethod, data) VALUES (?, ?, ?, ?, ?, ?)",
		p.PostId, length, fileName, mimeType, stgmethod, realData)
	// Generate an audit record.
	ar = AmNewAudit(AuditConferenceUploadAttachment, u.Uid, ipaddr, fmt.Sprintf("post=%d", p.PostId),
		fmt.Sprintf("len=%d,type=%s,name=%s,method=%d", length, mimeType, fileName, stgmethod))
	return err
}

// HitAttachment records a "hit" on an attachment.
func (p *PostHeader) HitAttachment(ctx context.Context) error {
	if p.ScribbleDate != nil && p.ScribbleUid != nil {
		return errors.New("no attachment on scribbled post")
	}
	_, err := amdb.ExecContext(ctx, "UPDATE postattach SET hits = hits + 1, lasthit = NOW() WHERE postid = ?", p.PostId)
	return err
}

// Text returns the text associated with a post.
func (p *PostHeader) Text(ctx context.Context) (string, error) {
	var dbdata []PostData
	if err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM postdata WHERE postid = ?", p.PostId); err != nil {
		return "", err
	}
	if len(dbdata) > 1 {
		return "", fmt.Errorf("too many data records (%d) for post #%d", len(dbdata), p.PostId)
	}
	if len(dbdata) == 0 || dbdata[0].Data == nil {
		return "", ErrNoPostData
	}
	return *dbdata[0].Data, nil
}

// Link returns a link string to this post.
func (p *PostHeader) Link(ctx context.Context, scope string) (string, error) {
	if scope == "topic" {
		return fmt.Sprintf("%d", p.Num), nil
	}
	if scope == "conference" || scope == "community" || scope == "global" {
		topic, err := AmGetTopic(ctx, p.TopicId)
		if err != nil {
			return "", err
		}
		parent, err := topic.Link(ctx, scope)
		if err != nil {
			return "", err
		}
		if strings.HasSuffix(parent, ".") {
			return fmt.Sprintf("%s%d", parent, p.Num), nil
		} else {
			return fmt.Sprintf("%s.%d", parent, p.Num), nil
		}
	}
	return "", errors.New("invalid scope")
}

// SetHidden sets the "hidden" flag on a post.
func (p *PostHeader) SetHidden(ctx context.Context, u *User, flag bool, ipaddr string) error {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	if p.ScribbleDate != nil && p.ScribbleUid != nil {
		return errors.New("cannot hide or unhide scribbled post")
	}
	if p.Hidden == flag {
		return nil // no-op
	}
	_, err := amdb.ExecContext(ctx, "UPDATE posts SET hidden = ? WHERE postid = ?", flag, p.PostId)
	if err == nil {
		p.Hidden = flag
		ar = AmNewAudit(AuditConferenceHideMessage, u.Uid, ipaddr, fmt.Sprintf("post=%d", p.PostId), fmt.Sprintf("hidden=%t", flag))
	}
	return err
}

// Scribble causes a post to be scribbled.
func (p *PostHeader) Scribble(ctx context.Context, u *User, ipaddr string) error {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	if p.ScribbleDate != nil && p.ScribbleUid != nil {
		return errors.New("cannot scribble an already-scribbled post")
	}

	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()
	// Scribble on the post header.
	scribblePseud := "<EM><B>(Scribbled)</B></EM>" // FUTURE: configurable option
	_, err := tx.ExecContext(ctx, "UPDATE posts SET linecount = 0, hidden = 0, scribble_uid = ?, scribble_date = NOW(), pseud = ? WHERE postid = ?", u.Uid, scribblePseud, p.PostId)
	if err != nil {
		return err
	}

	// Reread the scribble date.
	row := tx.QueryRowContext(ctx, "SELECT scribble_date FROM posts WHERE postid = ?", p.PostId)
	var newScribbleDate time.Time
	if err = row.Scan(&newScribbleDate); err != nil {
		return err
	}

	// Delete all auxiliary data.
	_, err = tx.ExecContext(ctx, "DELETE FROM postdata WHERE postid = ?", p.PostId)
	if err == nil {
		_, err = tx.ExecContext(ctx, "DELETE FROM postattach WHERE postid = ?", p.PostId)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM postpublish WHERE postid = ?", p.PostId)
		}
	}
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	success = true

	// Patch fields in the post header
	var newLines int32 = 0
	p.LineCount = &newLines
	p.Hidden = false
	newUid := u.Uid
	p.ScribbleUid = &newUid
	p.ScribbleDate = &newScribbleDate
	p.Pseud = &scribblePseud

	// Audit the operation.
	ar = AmNewAudit(AuditConferenceScribbleMessage, u.Uid, ipaddr, fmt.Sprintf("post=%d", p.PostId))
	return nil
}

// Nuke causes a post to be nuked (deleted entirely from the topic).
func (p *PostHeader) Nuke(ctx context.Context, u *User, ipaddr string) error {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// Delete all the references to this post.
	_, err := tx.ExecContext(ctx, "DELETE FROM posts WHERE postid = ?", p.PostId)
	if err == nil {
		_, err = tx.ExecContext(ctx, "DELETE FROM postdata WHERE postid = ?", p.PostId)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM postattach WHERE postid = ?", p.PostId)
			if err == nil {
				_, err = tx.ExecContext(ctx, "DELETE FROM postdogear WHERE postid = ?", p.PostId)
				if err == nil {
					_, err = tx.ExecContext(ctx, "DELETE FROM postpublish WHERE postid = ?", p.PostId)
				}
			}
		}
	}
	if err != nil {
		return err
	}

	// Renumber phase 1 - renumber posts in the same topic with a number higher than the nuked post
	if _, err = tx.ExecContext(ctx, "UPDATE posts SET num = (num - 1) WHERE topicid = ? AND num > ?", p.TopicId, p.Num); err != nil {
		return err
	}
	row := tx.QueryRowContext(ctx, "SELECT top_message FROM topics WHERE topicid = ?", p.TopicId)
	// Renumber phase 2 - reset the top message in this topic
	var topMessage int32
	if err = row.Scan(&topMessage); err != nil {
		return err
	}
	topMessage--
	if _, err = tx.ExecContext(ctx, "UPDATE topics SET top_message = ? WHERE topicid = ?", topMessage, p.TopicId); err != nil {
		return err
	}
	// Renumber phase 3 - adjust the last message in all settings for that topic
	if _, err = tx.ExecContext(ctx, "UPDATE topicsettings SET last_message = ? WHERE topicid = ? AND last_message > ?",
		topMessage, p.TopicId, topMessage); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	success = true
	ar = AmNewAudit(AuditConferenceNukeMessage, u.Uid, ipaddr, fmt.Sprintf("post=%d", p.PostId))
	return nil
}

// Publish publishes this message to the front page.
func (p *PostHeader) Publish(ctx context.Context, comm *Community, publisher *User, ipaddr string) error {
	if p.ScribbleDate != nil && p.ScribbleUid != nil {
		return errors.New("cannot publish scribbled post")
	}

	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// Check if we were already published.
	row := tx.QueryRowContext(ctx, "SELECT by_uid FROM postpublish WHERE postid = ?", p.PostId)
	var tmp int32
	err := row.Scan(&tmp)
	if err == nil {
		return errors.New("post already published")
	} else if err != sql.ErrNoRows {
		return err
	}

	// Publish it!
	if _, err = tx.ExecContext(ctx, "INSERT INTO postpublish (commid, postid, by_uid, on_date) VALUES (?, ?, ?, NOW())",
		comm.Id, p.PostId, publisher.Uid); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	success = true
	ar = AmNewAudit(AuditPublishToFrontPage, publisher.Uid, ipaddr, fmt.Sprintf("comm=%d,post=%d", comm.Id, p.PostId))
	return nil
}

// MoveTo moves this message to a new topic.
func (p *PostHeader) MoveTo(ctx context.Context, target *Topic, u *User, ipaddr string) error {
	if target.TopicId == p.TopicId {
		return nil // this is a no-op
	}
	if p.ScribbleDate != nil && p.ScribbleUid != nil {
		return errors.New("cannot move a scribbled message")
	}

	oldTopic, err := AmGetTopic(ctx, p.TopicId)
	if err != nil {
		return err
	}
	if oldTopic.ConfId != target.ConfId {
		return errors.New("target topic must be in the same conference")
	}
	if oldTopic.TopMessage == 0 {
		return errors.New("cannot move the only message out of a conference")
	}
	conf, err := AmGetConference(ctx, oldTopic.ConfId)
	if err != nil {
		return err
	}

	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// Adjust post record in the database to make it part of the new topic.
	_, err = tx.ExecContext(ctx, "UPDATE posts SET parent = 0, topicid = ?, num = ? WHERE postid = ?", target.TopicId, target.TopMessage+1, p.PostId)
	if err != nil {
		return err
	}
	// Adjust the topic values in the database to reflect that it has a new post.
	_, err = tx.ExecContext(ctx, "UPDATE topics SET top_message = top_message + 1, lastupdate = NOW() WHERE topicid = ?", target.TopicId)
	if err != nil {
		return err
	}
	// Read back the last update.
	row := tx.QueryRowContext(ctx, "SELECT lastupdate FROM topics WHERE topicid = ?", target.TopicId)
	var lastUpdate time.Time
	err = row.Scan(&lastUpdate)
	if err != nil {
		return err
	}

	// Now we have to renumber the posts in the OLD topic just as if the old post was nuked.
	_, err = tx.ExecContext(ctx, "UPDATE posts SET num = num - 1 WHERE topicid = ? AND num > ?", p.TopicId, p.Num)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE topics SET top_message = top_message - 1 WHERE topicid = ?", p.TopicId)
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE posts SET parent = ? WHERE parent = ?", p.Parent, p.PostId)
			if err == nil {
				_, err = tx.ExecContext(ctx, "UPDATE topicsettings SET last_message = ? WHERE topicid = ? AND last_message > ?",
					oldTopic.TopMessage-1, p.TopicId, oldTopic.TopMessage-1)
			}
		}
	}
	if err != nil {
		return err
	}

	// Touch the "update" in the conference.
	err = conf.TouchUpdate(ctx, tx, lastUpdate)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	success = true

	// Now patch the data structures we have.
	p.Parent = 0
	p.TopicId = target.TopicId
	p.Num = target.TopMessage + 1
	target.TopMessage++
	target.LastUpdate = lastUpdate

	// And audit the result.
	ar = AmNewAudit(AuditConferenceMoveMessage, u.Uid, ipaddr, fmt.Sprintf("conf=%d,post=%d", conf.ConfId, p.PostId),
		fmt.Sprintf("fromTopic=%d", oldTopic.TopicId), fmt.Sprintf("toTopic=%d", target.TopicId))
	return nil
}

/* AmGetPost gets a single post from the database by ID.
 * Parameters:
 *     ctx - Standard Go context value.
 *     postId - ID of the post to retrieve.
 * Returns:
 *     Pointer to PostHeader for the post, or nil.
 *     Standard Go error status.
 */
func AmGetPost(ctx context.Context, postId int64) (*PostHeader, error) {
	var dbdata []PostHeader
	if err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM posts WHERE postid = ?", postId); err != nil {
		return nil, err
	}
	if len(dbdata) == 0 {
		return nil, errors.New("post not found")
	}
	if len(dbdata) > 1 {
		return nil, fmt.Errorf("AmGetPost: too many entries (%d) for post ID %d", len(dbdata), postId)
	}
	return &(dbdata[0]), nil
}

/* AmGetPostRange gets a range of posts from a topic by post numbers.
 * Parameters:
 *     ctx - Standard Go context value.
 *     topic - Topic pointer to retrieve posts from.
 *     first - Number of first post to retrieve.
 *     last - Number of last post to retrieve.
 * Returns:
 *     Array of pointers to PostHeader objects, or nil.
 *     Standard Go error status.
 */
func AmGetPostRange(ctx context.Context, topic *Topic, first, last int32) ([]*PostHeader, error) {
	var posts []PostHeader
	if err := amdb.SelectContext(ctx, &posts, "SELECT * FROM posts WHERE topicid = ? AND num >= ? AND num <= ? ORDER BY num", topic.TopicId, first, last); err != nil {
		return nil, err
	}
	rc := make([]*PostHeader, len(posts))
	for i := range posts {
		rc[i] = &(posts[i])
	}
	return rc, nil
}

/* AmNewPost adds a new post to a topic.
 * Parameters:
 *     ctx - Standard Go context value.
 *     conf - Pointer to conference containing the topic.
 *     topic - Pointer to topic.
 *     user - Pointer to user posting the message.
 *     pseud - Pseud for the new post.
 *     post - New post text.
 *     postLines - Number of lines in the post text.
 *     ipaddr - IP address of user maing the post.
 * Returns:
 *     New post header pointer.
 *     Standard Go error status.
 */
func AmNewPost(ctx context.Context, conf *Conference, topic *Topic, user *User, pseud string, post string, postLines int32, ipaddr string) (*PostHeader, error) {
	success := false
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// Add the post header information.
	rs, err := tx.ExecContext(ctx, "INSERT INTO posts (topicid, num, linecount, creator_uid, posted, pseud) VALUES (?, ?, ?, ?, NOW(), ?)",
		topic.TopicId, topic.TopMessage+1, postLines, user.Uid, pseud)
	if err != nil {
		return nil, err
	}
	xid, err := rs.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Read back the post header.
	var dbdata []PostHeader
	if err := tx.SelectContext(ctx, &dbdata, "SELECT * FROM posts WHERE postid = ?", xid); err != nil {
		return nil, err
	}
	if len(dbdata) == 0 {
		return nil, errors.New("AmNewPost: new post not found")
	}
	if len(dbdata) > 1 {
		return nil, fmt.Errorf("AmNewPost: too many entries (%d) for post ID %d", len(dbdata), xid)
	}
	hdr := &(dbdata[0])

	// Add the post data.
	_, err = tx.ExecContext(ctx, "INSERT INTO postdata (postid, data) VALUES (?, ?)", hdr.PostId, post)
	if err != nil {
		return nil, err
	}

	// Update the topic.
	_, err = tx.ExecContext(ctx, "UPDATE topics SET top_message = ?, lastupdate = ? WHERE topicid = ?", hdr.Num, hdr.Posted, topic.TopicId)
	if err != nil {
		return nil, err
	}
	topic.TopMessage = hdr.Num
	topic.LastUpdate = hdr.Posted

	// update the "last update" date of the conference and the "last posted" date in the conference settings
	if err = conf.TouchUpdate(ctx, tx, hdr.Posted); err != nil {
		return nil, err
	}
	_, err = conf.TouchPost(ctx, tx, user, hdr.Posted)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	success = true

	// create audit record
	ar = AmNewAudit(AuditConferencePostMessage, user.Uid, ipaddr, fmt.Sprintf("confid=%d", conf.ConfId),
		fmt.Sprintf("topic=%d", topic.Number), fmt.Sprintf("post=%d", hdr.PostId), fmt.Sprintf("pseud=%s", *hdr.Pseud))

	return hdr, nil
}

/* AmGetPublishedPosts gets all posts published to the front page, up to the maximum number configured in the database.
 * Parameters:
 *     ctx - Standard Go context value.
 * Returns:
 *     Array of post headers, or nil.
 *     Standard Go error status.
 */
func AmGetPublishedPosts(ctx context.Context) ([]*PostHeader, error) {
	// Read the globals.
	gv, err := AmGlobals(ctx)
	if err != nil {
		return nil, err
	}
	// Read the published posts.
	rs, err := amdb.QueryContext(ctx, "SELECT postid FROM postpublish ORDER BY on_date DESC")
	if err != nil {
		return nil, err
	}
	// Extract post IDs to an array.
	pids := make([]int64, gv.FrontPagePosts)
	i := 0
	for i < int(gv.FrontPagePosts) && rs.Next() {
		if err = rs.Scan(&(pids[i])); err != nil {
			return nil, err
		}
		i++
	}
	if i == 0 { // no published posts, short-circuit response
		return make([]*PostHeader, 0), nil
	}
	if i < int(gv.FrontPagePosts) {
		pids = pids[:i] // truncate if we have fewer posts than spaces
	}

	// Use the post IDs to build a SQL statement.
	pidStrs := make([]string, len(pids))
	for i, pid := range pids {
		pidStrs[i] = fmt.Sprintf("%d", pid)
	}
	sql := fmt.Sprintf("SELECT * FROM posts WHERE postid IN (%s)", strings.Join(pidStrs, ", "))

	// Use the SQL to read in all the post headers using a single database query.
	var data []PostHeader
	if err = amdb.SelectContext(ctx, &data, sql); err != nil {
		return nil, err
	}
	if len(data) < len(pids) {
		return nil, errors.New("internal error reading post headers")
	}

	// Build the return array by making sure we point to the post headers in the same order the post IDs were returned.
	rc := make([]*PostHeader, len(pids))
	q := 0
	for i := range data {
		for j := range pids {
			if data[i].PostId == pids[j] {
				rc[j] = &(data[i])
				q++
			}
		}
	}
	if q < len(pids) {
		return nil, errors.New("internal error generating output")
	}

	return rc, nil
}

type PostSearchResult struct {
	PostLink string
	Author   string
	PostDate time.Time
	Lines    int32
	Excerpt  string
}

const EXCERPT_MAX = 60 // temporary implementation

// decodeSearchScope turns the scope values from the AmSearchPosts call into a set of coherent values.
func decodeSearchScope(ctx context.Context, scopeValues []any) (string, *Community, *Conference, *Topic, error) {
	var myComm *Community = nil
	var myConf *Conference = nil
	var myTopic *Topic = nil

	// Sort the items in the scopeValues array and fill them in the right slots.
	for i := range scopeValues {
		if scopeValues[i] == nil {
			continue
		}
		if thisComm, ok := scopeValues[i].(*Community); ok {
			if myComm != nil {
				return "error", nil, nil, nil, errors.New("cannot specify multiple communities")
			}
			myComm = thisComm
			continue
		}
		if thisConf, ok := scopeValues[i].(*Conference); ok {
			if myConf != nil {
				return "error", nil, nil, nil, errors.New("cannot specify multiple conferences")
			}
			myConf = thisConf
			continue
		}
		if thisTopic, ok := scopeValues[i].(*Topic); ok {
			if myTopic != nil {
				return "error", nil, nil, nil, errors.New("cannot specify multiple topics")
			}
			myTopic = thisTopic
			continue
		}
		return "error", nil, nil, nil, errors.New("invalid item specified in scope")
	}

	// Based on which slots are full, determine the scope. Also error-check relations between the specified slots.
	if myComm == nil {
		if myConf != nil || myTopic != nil {
			return "error", nil, nil, nil, errors.New("conference/topic specified without community")
		}
		return "global", nil, nil, nil, nil
	}
	if myConf == nil {
		if myTopic != nil {
			return "error", nil, nil, nil, errors.New("topic specified without conference")
		}
		return "community", myComm, nil, nil, nil
	}
	f, err := myConf.InCommunity(ctx, myComm)
	if err != nil {
		return "error", nil, nil, nil, err
	}
	if !f {
		return "error", nil, nil, nil, errors.New("community does not contain conference")
	}
	if myTopic == nil {
		return "conference", myComm, myConf, nil, nil
	}
	if myTopic.ConfId != myConf.ConfId {
		return "error", nil, nil, nil, errors.New("conference does not contain topic")
	}
	return "topic", myComm, myConf, myTopic, nil
}

/* AmSearchPosts finds posts by using full text search on their contents.
 * Parameters:
 *     ctx - Standard Go context value.
 *     searchTerms - The terms to search for in the text.
 *     u - The user performing the search.
 *     offset - How many posts in the results to skip.
 *     max - Maximum number of posts to return.
 *     scopeValues - Multiple object values to limit the scope. Put a Community pointer here to limit the scope to
 *                   that community. Also add a Conference pointer (from that community) to limit the scope to that conference.
 *                   Also add a Topic pointer (from that conference) to limit the scope to that topic.
 * Returns:
 *     Array of PostSearchResult structures with the results.
 *     Total number of posts that match the search.
 *     Standard Go error status.
 */
func AmSearchPosts(ctx context.Context, searchTerms string, u *User, offset, max int, scopeValues ...any) ([]PostSearchResult, int, error) {
	// Decode the search scope.
	scope, comm, conf, topic, err := decodeSearchScope(ctx, scopeValues)
	if err != nil {
		return nil, -1, err
	}

	// Get the proper service index to match against the community services.
	confService, err := AmGetServiceIndex("community", "Conference")
	if err != nil {
		return nil, -1, err
	}

	// Get the count of matching posts.
	var row *sql.Row
	switch scope {
	case "global":
		row = amdb.QueryRowContext(ctx, `SELECT COUNT(*)
			FROM communities q JOIN commtoconf s ON s.commid = q.commid JOIN confs c ON c.confid = s.confid
			JOIN commmember m ON m.commid = q.commid JOIN users u ON u.uid = m.uid JOIN commftrs f ON f.commid = q.commid
			JOIN topics t ON t.confid = c.confid JOIN posts p ON p.topicid = t.topicid JOIN postdata d ON d.postid = p.postid JOIN users u2 ON u2.uid = p.creator_uid
			LEFT JOIN confmember x ON (c.confid = x.confid AND u.uid = x.uid)
			WHERE u.uid = ? AND f.ftr_code = ? AND GREATEST(u.base_lvl,m.granted_lvl,s.granted_lvl,IFNULL(x.granted_lvl,0)) >= c.read_lvl
			AND p.scribble_uid IS NULL AND MATCH(d.data) AGAINST (?)`, u.Uid, confService, searchTerms)
	case "community":
		row = amdb.QueryRowContext(ctx, `SELECT COUNT(*)
			FROM communities q JOIN commtoconf s ON s.commid = q.commid JOIN confs c ON c.confid = s.confid
			JOIN commmember m ON m.commid = q.commid JOIN users u ON u.uid = m.uid JOIN commftrs f ON f.commid = q.commid
			JOIN topics t ON t.confid = c.confid JOIN posts p ON p.topicid = t.topicid JOIN postdata d ON d.postid = p.postid JOIN users u2 ON u2.uid = p.creator_uid
			LEFT JOIN confmember x ON (c.confid = x.confid AND u.uid = x.uid)
			WHERE u.uid = ? AND f.ftr_code = ? AND GREATEST(u.base_lvl,m.granted_lvl,s.granted_lvl,IFNULL(x.granted_lvl,0)) >= c.read_lvl
			AND q.commid = ? AND p.scribble_uid IS NULL AND MATCH(d.data) AGAINST (?)`, u.Uid, confService, comm.Id, searchTerms)
	case "conference":
		row = amdb.QueryRowContext(ctx, `SELECT COUNT(*)
			FROM communities q JOIN commtoconf s ON s.commid = q.commid JOIN confs c ON c.confid = s.confid
			JOIN commmember m ON m.commid = q.commid JOIN users u ON u.uid = m.uid JOIN commftrs f ON f.commid = q.commid
			JOIN topics t ON t.confid = c.confid JOIN posts p ON p.topicid = t.topicid JOIN postdata d ON d.postid = p.postid JOIN users u2 ON u2.uid = p.creator_uid
			LEFT JOIN confmember x ON (c.confid = x.confid AND u.uid = x.uid)
			WHERE u.uid = ? AND f.ftr_code = ? AND GREATEST(u.base_lvl,m.granted_lvl,s.granted_lvl,IFNULL(x.granted_lvl,0)) >= c.read_lvl
			AND q.commid = ? AND c.confid = ? AND p.scribble_uid IS NULL AND MATCH(d.data) AGAINST (?)`, u.Uid, confService, comm.Id, conf.ConfId, searchTerms)
	case "topic":
		row = amdb.QueryRowContext(ctx, `SELECT COUNT(*)
			FROM communities q JOIN commtoconf s ON s.commid = q.commid JOIN confs c ON c.confid = s.confid
			JOIN commmember m ON m.commid = q.commid JOIN users u ON u.uid = m.uid JOIN commftrs f ON f.commid = q.commid
			JOIN topics t ON t.confid = c.confid JOIN posts p ON p.topicid = t.topicid JOIN postdata d ON d.postid = p.postid JOIN users u2 ON u2.uid = p.creator_uid
			LEFT JOIN confmember x ON (c.confid = x.confid AND u.uid = x.uid)
			WHERE u.uid = ? AND f.ftr_code = ? AND GREATEST(u.base_lvl,m.granted_lvl,s.granted_lvl,IFNULL(x.granted_lvl,0)) >= c.read_lvl
			AND q.commid = ? AND c.confid = ? AND t.topicid = ? AND p.scribble_uid IS NULL AND MATCH(d.data) AGAINST (?)`,
			u.Uid, confService, comm.Id, conf.ConfId, topic.TopicId, searchTerms)
	}
	var count int
	err = row.Scan(&count)
	if err != nil {
		log.Errorf("AmSearchPosts query 1 error %v", err)
		return nil, -1, err
	}

	// Get the matching posts themselves.
	var rs *sql.Rows
	switch scope {
	case "global":
		rs, err = amdb.QueryContext(ctx, `SELECT q.commid, q.alias, c.confid, t.topicid, t.num, p.postid, p.num, u2.username, p.posted, p.linecount, d.data
			FROM communities q JOIN commtoconf s ON s.commid = q.commid JOIN confs c ON c.confid = s.confid
			JOIN commmember m ON m.commid = q.commid JOIN users u ON u.uid = m.uid JOIN commftrs f ON f.commid = q.commid
			JOIN topics t ON t.confid = c.confid JOIN posts p ON p.topicid = t.topicid JOIN postdata d ON d.postid = p.postid JOIN users u2 ON u2.uid = p.creator_uid
			LEFT JOIN confmember x ON (c.confid = x.confid AND u.uid = x.uid)
			WHERE u.uid = ? AND f.ftr_code = ? AND GREATEST(u.base_lvl,m.granted_lvl,s.granted_lvl,IFNULL(x.granted_lvl,0)) >= c.read_lvl
			AND p.scribble_uid IS NULL AND MATCH(d.data) AGAINST (?) ORDER BY q.commname, c.name, t.num, p.num
			LIMIT ? OFFSET ?`, u.Uid, confService, searchTerms, max, offset)
	case "community":
		rs, err = amdb.QueryContext(ctx, `SELECT q.commid, q.alias, c.confid, t.topicid, t.num, p.postid, p.num, u2.username, p.posted, p.linecount, d.data
			FROM communities q JOIN commtoconf s ON s.commid = q.commid JOIN confs c ON c.confid = s.confid
			JOIN commmember m ON m.commid = q.commid JOIN users u ON u.uid = m.uid JOIN commftrs f ON f.commid = q.commid
			JOIN topics t ON t.confid = c.confid JOIN posts p ON p.topicid = t.topicid JOIN postdata d ON d.postid = p.postid JOIN users u2 ON u2.uid = p.creator_uid
			LEFT JOIN confmember x ON (c.confid = x.confid AND u.uid = x.uid)
			WHERE u.uid = ? AND f.ftr_code = ? AND GREATEST(u.base_lvl,m.granted_lvl,s.granted_lvl,IFNULL(x.granted_lvl,0)) >= c.read_lvl
			AND q.commid = ? AND p.scribble_uid IS NULL AND MATCH(d.data) AGAINST (?) ORDER BY q.commname, c.name, t.num, p.num
			LIMIT ? OFFSET ?`, u.Uid, confService, comm.Id, searchTerms, max, offset)
	case "conference":
		rs, err = amdb.QueryContext(ctx, `SELECT q.commid, q.alias, c.confid, t.topicid, t.num, p.postid, p.num, u2.username, p.posted, p.linecount, d.data
			FROM communities q JOIN commtoconf s ON s.commid = q.commid JOIN confs c ON c.confid = s.confid
			JOIN commmember m ON m.commid = q.commid JOIN users u ON u.uid = m.uid JOIN commftrs f ON f.commid = q.commid
			JOIN topics t ON t.confid = c.confid JOIN posts p ON p.topicid = t.topicid JOIN postdata d ON d.postid = p.postid JOIN users u2 ON u2.uid = p.creator_uid
			LEFT JOIN confmember x ON (c.confid = x.confid AND u.uid = x.uid)
			WHERE u.uid = ? AND f.ftr_code = ? AND GREATEST(u.base_lvl,m.granted_lvl,s.granted_lvl,IFNULL(x.granted_lvl,0)) >= c.read_lvl
			AND q.commid = ? AND c.confid = ? AND p.scribble_uid IS NULL AND MATCH(d.data) AGAINST (?) ORDER BY q.commname, c.name, t.num, p.num
			LIMIT ? OFFSET ?`, u.Uid, confService, comm.Id, conf.ConfId, searchTerms, max, offset)
	case "topic":
		rs, err = amdb.QueryContext(ctx, `SELECT q.commid, q.alias, c.confid, t.topicid, t.num, p.postid, p.num, u2.username, p.posted, p.linecount, d.data
			FROM communities q JOIN commtoconf s ON s.commid = q.commid JOIN confs c ON c.confid = s.confid
			JOIN commmember m ON m.commid = q.commid JOIN users u ON u.uid = m.uid JOIN commftrs f ON f.commid = q.commid
			JOIN topics t ON t.confid = c.confid JOIN posts p ON p.topicid = t.topicid JOIN postdata d ON d.postid = p.postid JOIN users u2 ON u2.uid = p.creator_uid
			LEFT JOIN confmember x ON (c.confid = x.confid AND u.uid = x.uid)
			WHERE u.uid = ? AND f.ftr_code = ? AND GREATEST(u.base_lvl,m.granted_lvl,s.granted_lvl,IFNULL(x.granted_lvl,0)) >= c.read_lvl
			AND q.commid = ? AND c.confid = ? AND t.topicid = ? AND p.scribble_uid IS NULL AND MATCH(d.data) AGAINST (?) ORDER BY q.commname, c.name, t.num, p.num
			LIMIT ? OFFSET ?`, u.Uid, confService, comm.Id, conf.ConfId, topic.TopicId, searchTerms, max, offset)
	}
	if err != nil {
		log.Errorf("AmSearchPosts query 2 error %v", err)
		return nil, count, err
	}
	rc := make([]PostSearchResult, max)
	i := 0
	for rs.Next() {
		var commid int32
		var commAlias string
		var confid int32
		var topicid int32
		var topicNum int16
		var postid int64
		var postnum int32
		err := rs.Scan(&commid, &commAlias, &confid, &topicid, &topicNum, &postid, &postnum, &(rc[i].Author), &(rc[i].PostDate),
			&(rc[i].Lines), &(rc[i].Excerpt))
		if err != nil {
			return nil, count, err
		}

		// Get conference so we can get aliases.
		conf, err := AmGetConference(ctx, confid)
		if err != nil {
			return nil, count, err
		}
		alias, err := conf.Aliases(ctx)
		if err != nil {
			return nil, count, err
		}

		// Build the post link.
		plink := AmCreatePostLinkContext(commAlias, alias[0], topicNum)
		plink.FirstPost = postnum
		plink.LastPost = postnum
		rc[i].PostLink = plink.AsString()

		// Trim down the excerpt.
		if len(rc[i].Excerpt) > EXCERPT_MAX {
			choplen := min(len(rc[i].Excerpt), EXCERPT_MAX*3)
			tmp := []rune(rc[i].Excerpt[:choplen])
			choplen = min(len(tmp), EXCERPT_MAX)
			rc[i].Excerpt = fmt.Sprintf("%s...", string(tmp[:choplen]))
		}
		i++ // go on to the next
	}

	if i < max {
		rc = rc[:i] // slice off any empty entries at the end
	}
	return rc, count, nil
}
