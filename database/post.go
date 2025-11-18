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

func (p *PostHeader) SetAttachment(fileName string, mimeType string, length int32, data []byte) error {
	_, err := amdb.Exec("INSERT INTO postattach (postid, datalen, filename, mimetype, data) VALUES (?, ?, ?, ?, ?)",
		p.PostId, length, fileName, mimeType, data)
	return err
}

func AmGetPost(postId int64) (*PostHeader, error) {
	var dbdata []PostHeader
	err := amdb.Select(&dbdata, "SELECT * FROM posts WHERE postid = ?", postId)
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
