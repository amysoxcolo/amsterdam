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
	"context"
	"errors"
	"fmt"
	"time"
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

// ErrNoPostData is returned if post data is missing.
var ErrNoPostData = errors.New("no post data")

// IsScribbled returns true if the post has been scribbled, false if not.
func (p *PostHeader) IsScribbled() bool {
	return p.ScribbleUid != nil && p.ScribbleDate != nil
}

// IsPublished returns true if the post has been published to the front page.
func (p *PostHeader) IsPublished(ctx context.Context) (bool, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT COUNT(*) FROM postpublish WHERE postid = ?", p.PostId)
	if err != nil {
		return false, err
	}
	if !rs.Next() {
		return false, errors.New("internal failure in IsPublished")
	}
	ct := 0
	err = rs.Scan(&ct)
	return ct > 0, err
}

/* AttachmentInfo returns attachment information for a post.
 * Parameters:
 *     ctx - Standard Go context value.
 * Returns:
 *     Pointer to structure with post attachment info.
 *     Standard Go error status.
 */
func (p *PostHeader) AttachmentInfo(ctx context.Context) (*PostAttachInfo, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT filename, mimetype, datalen FROM postattach WHERE postid = ?", p.PostId)
	if err != nil {
		return nil, err
	}
	if !rs.Next() {
		return nil, nil
	}
	var rc PostAttachInfo
	err = rs.Scan(&(rc.Filename), &(rc.MIMEType), &(rc.Length))
	return &rc, err
}

/* SetAttachment sets the attachment data for a post.
 * Parameters:
 *     ctx - Standard Go context value.
 *     fileName - Name of the original attachment file.
 *     mimeType - MIME type of the attachment data.
 *     length - Length of the attachment data in bytes.
 *     data - The attachment data itself.
 * Returns:
 *     Standard Go error status.
 */
func (p *PostHeader) SetAttachment(ctx context.Context, fileName string, mimeType string, length int32, data []byte) error {
	_, err := amdb.ExecContext(ctx, "INSERT INTO postattach (postid, datalen, filename, mimetype, data) VALUES (?, ?, ?, ?, ?)",
		p.PostId, length, fileName, mimeType, data)
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

/* AmGetPostRage gets a range of posts from a topic by post numbers.
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
	unlock := true
	tx.ExecContext(ctx, "LOCK TABLES confs WRITE, topics WRITE, topicsettings WRITE, posts WRITE, postdata WRITE;")
	defer func() {
		if unlock {
			tx.ExecContext(ctx, "UNLOCK TABLES;")
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

	tx.ExecContext(ctx, "UNLOCK TABLES;")
	unlock = false

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
