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

var ErrNoPostData = errors.New("no post data")

// IsScribbled returns true if the post has been scribbled, false if not.
func (p *PostHeader) IsScribbled() bool {
	return p.ScribbleUid != nil && p.ScribbleDate != nil
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
	err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM postdata WHERE postid = ?", p.PostId)
	if err != nil {
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

func AmGetPost(ctx context.Context, postId int64) (*PostHeader, error) {
	var dbdata []PostHeader
	err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM posts WHERE postid = ?", postId)
	if err != nil {
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

func AmGetPostRange(ctx context.Context, topic *Topic, first, last int32) ([]PostHeader, error) {
	var rc []PostHeader
	err := amdb.SelectContext(ctx, &rc, "SELECT * FROM posts WHERE topicid = ? AND num >= ? AND num <= ? ORDER BY num", topic.TopicId, first, last)
	if err != nil {
		return nil, err
	}
	return rc, nil
}
