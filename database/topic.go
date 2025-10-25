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

import "time"

type Topic struct {
	TopicId    int32     `db:"topicid"`
	ConfId     int32     `db:"confid"`
	Number     int16     `db:"num"`
	CreatorUid int32     `db:"creator_uid"`
	TopMessage int32     `db:"top_message"`
	Frozen     bool      `db:"frozen"`
	Archived   bool      `db:"archived"`
	Sticky     bool      `db:"sticky"`
	CreateDate time.Time `db:"createdate"`
	LastUpdate time.Time `db:"lastupdate"`
	Name       string    `db:"name"`
}

type TopicSettings struct {
	TopicId     int32      `db:"topicid"`
	Uid         int32      `db:"uid"`
	Hidden      bool       `db:"hidden"`
	LastMessage int32      `db:"last_message"`
	LastRead    *time.Time `db:"last_read"`
	LastPost    *time.Time `db:"last_post"`
	Subscribe   bool       `db:"subscribe"`
}
