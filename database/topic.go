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
	"strings"
	"time"
)

// Topic is the top-level structure detailing topics.
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

// TopicSettings contains per-user settings for topics, including the "last read" message pointer.
type TopicSettings struct {
	TopicId     int32      `db:"topicid"`
	Uid         int32      `db:"uid"`
	Hidden      bool       `db:"hidden"`
	LastMessage int32      `db:"last_message"`
	LastRead    *time.Time `db:"last_read"`
	LastPost    *time.Time `db:"last_post"`
	Subscribe   bool       `db:"subscribe"`
}

// TopicSummary is a smaller data structure that gets topic information to create the topic list display.
type TopicSummary struct {
	TopicID    int32
	Number     int16
	Name       string
	Unread     int32
	Total      int32
	LastUpdate time.Time
	Frozen     bool
	Archived   bool
	Subscribed bool
}

func AmGetTopic(topicId int32) (*Topic, error) {
	var dbdata []Topic
	err := amdb.Select(&dbdata, "SELECT * FROM topics WHERE topicid = ?", topicId)
	if err != nil {
		return nil, err
	}
	if len(dbdata) == 0 {
		return nil, fmt.Errorf("topic %d not found", topicId)
	}
	if len(dbdata) > 1 {
		return nil, fmt.Errorf("AmGetTopic(%d): too many responses (%d)", topicId, len(dbdata))
	}
	return &(dbdata[0]), nil
}

// View and sort constants for AmListTopics.
const (
	TopicViewAll        = 0
	TopicViewNew        = 1
	TopicViewActive     = 2
	TopicViewAllVisible = 3
	TopicViewHidden     = 4
	TopicViewArchive    = 5

	TopicSortID     = 0
	TopicSortNumber = 1
	TopicSortName   = 2
	TopicSortUnread = 3
	TopicSortTotal  = 4
	TopicSortDate   = 5
)

/* AmListTopics produces a list of topic summary information according to specific options.
 * Parameters:
 *     confid - The ID of the conference to list topics in.
 *     uid - The UID of the user to consider the settings of.
 *     viewOption - One of the following constants:
 *         TopicViewAll - List all topics.
 *         TopicViewNew - List only visible topics with new messages.
 *         TopicViewActive - List only visible topics, with "active" ones coming first.
 *         TopicViewAllVisible - List only visible topics.
 *		   TopicViewHidden - List only hidden topics (including archived ones).
 *         TopicViewArchive - List only archived, non-hidden topics.
 *     sortOption - One of the following constants:
 *         TopicSortID - Sort by topic ID.
 *         TopicSortNumber - Sort by topic number in the conference. May be negated to sort in reverse order.
 *         TopicSortName - Sort by topic name. May be negated to sort in reverse order.
 *         TopicSortUnread - Sort by number of unread messages. May be negated to sort in reverse order.
 *         TopicSortTotal - Sort by total number of messages. May be negated to sort in reverse order.
 *         TopicSortDate - Sort by last topic update date. May be negated to sort in reverse order.
 *     ignoreSticky - If false, sticky topics will precede nonsticky ones; if true, stickiness is ignored.
 * Returns:
 *     List of TopicSummary pointers.
 *     Standard Go error status.
 */
func AmListTopics(confid int32, uid int32, viewOption int, sortOption int, ignoreSticky bool) ([]*TopicSummary, error) {
	// Decode the viewOption into a WHERE clause.
	var whereClause string
	switch viewOption {
	case TopicViewAll:
		whereClause = ""
	case TopicViewNew:
		tail := "t.top_message > IFNULL(s.last_message,-1)"
		if !ignoreSticky {
			tail = "(t.sticky = 1 OR " + tail + ")"
		}
		whereClause = "t.archived = 0 AND IFNULL(s.hidden,0) = 0 AND " + tail
	case TopicViewActive, TopicViewAllVisible:
		whereClause = "t.archived = 0 AND IFNULL(s.hidden,0) = 0"
	case TopicViewHidden:
		whereClause = "IFNULL(s.hidden,0) = 1"
	case TopicViewArchive:
		whereClause = "t.archived = 1 AND IFNULL(s.hidden,0) = 0"
	default:
		return nil, errors.New("invalid view option specified")
	}

	// Decode the sortOption into an ORDER BY clause.
	var reverse bool = false
	if sortOption < 0 {
		reverse = true
		sortOption = -sortOption
	}
	var orderByClause string
	switch sortOption {
	case TopicSortID:
		orderByClause = "t.topicid ASC"
	case TopicSortNumber:
		if reverse {
			orderByClause = "t.num DESC"
		} else {
			orderByClause = "t.num ASC"
		}
	case TopicSortName:
		if reverse {
			orderByClause = "t.name DESC, t.num DESC"
		} else {
			orderByClause = "t.name ASC, t.num ASC"
		}
	case TopicSortUnread:
		if reverse {
			orderByClause = "unread ASC, t.num DESC"
		} else {
			orderByClause = "unread DESC, t.num ASC"
		}
	case TopicSortTotal:
		if reverse {
			orderByClause = "total ASC, t.num DESC"
		} else {
			orderByClause = "total DESC, t.num ASC"
		}
	case TopicSortDate:
		if reverse {
			orderByClause = "t.lastupdate ASC, t.num DESC"
		} else {
			orderByClause = "t.lastupdate DESC, t.num ASC"
		}
	default:
		return nil, errors.New("invalid sort option specified")
	}

	// Build the full SQL statement
	var fullStatement strings.Builder
	fullStatement.WriteString("SELECT t.topicid, t.num, t.name, (t.top_message - IFNULL(s.last_message,-1)) AS unread, ")
	fullStatement.WriteString("(t.top_message + 1) AS total, t.lastupdate, t.frozen, t.archived, IFNULL(s.subscribe,0) AS subscribe, ")
	fullStatement.WriteString("t.sticky, GREATEST(SIGN(t.top_message - IFNULL(s.last_message,-1)),0) AS newflag ")
	fullStatement.WriteString("FROM topics t LEFT JOIN topicsettings s ON t.topicid = s.topicid AND s.uid = ? WHERE t.confid = ? ")
	if whereClause != "" {
		fullStatement.WriteString("AND ")
		fullStatement.WriteString(whereClause)
	}
	fullStatement.WriteString(" ORDER BY ")
	if ignoreSticky {
		fullStatement.WriteString("t.sticky DESC, ")
	}
	if viewOption == TopicViewActive {
		fullStatement.WriteString("newflag DESC, ")
	}
	fullStatement.WriteString(orderByClause)

	// Execute and capture results
	rs, err := amdb.Query(fullStatement.String(), uid, confid)
	if err != nil {
		return nil, err
	}
	rc := make([]*TopicSummary, 0)
	for rs.Next() {
		var rec TopicSummary
		rs.Scan(&rec.TopicID, &rec.Number, &rec.Name, &rec.Unread, &rec.Total, &rec.LastUpdate, &rec.Frozen, &rec.Archived,
			&rec.Subscribed)
		rc = append(rc, &rec)
	}
	return rc, nil
}

func AmNewTopic(conf *Conference, user *User, title string, zeroPostPseud string, zeroPost string, zeroPostLines int32) (*Topic, error) {
	unlock := true
	amdb.Exec("LOCK TABLES confs WRITE, topics WRITE, topicsettings WRITE, posts WRITE, postdata WRITE;")
	defer func() {
		if unlock {
			amdb.Exec("UNLOCK TABLES;")
		}
	}()

	// Insert the new topic into the database.
	conf.Mutex.Lock()
	rs, err := amdb.Exec("INSERT INTO topics (confid, num, creator_uid, createdate, lastupdate, name) VALUES (?, ?, ?, NOW(), NOW(), ?)",
		conf.ConfId, conf.TopTopic+1, user.Uid, title)
	if err != nil {
		conf.Mutex.Unlock()
		return nil, err
	}
	xid, err := rs.LastInsertId()
	if err != nil {
		conf.Mutex.Unlock()
		return nil, err
	}
	topic, err := AmGetTopic(int32(xid))
	if err != nil {
		conf.Mutex.Unlock()
		return nil, err
	}

	// Update the conference to set the last update and top topic.
	_, err = amdb.Exec("UPDATE confs SET lastupdate = ?, top_topic = ? WHERE confid = ?", topic.CreateDate, conf.TopTopic+1, conf.ConfId)
	if err != nil {
		conf.Mutex.Unlock()
		return nil, err
	}
	conf.TopTopic++
	conf.LastUpdate = &topic.CreateDate
	conf.Mutex.Unlock()

	// Add the "header record" for the first post.
	rs, err = amdb.Exec("INSERT INTO posts (topicid, num, linecount, creator_uid, posted, pseud) VALUES (?, 0, ?, ?, ?, ?)",
		topic.TopicId, zeroPostLines, user.Uid, topic.CreateDate, zeroPostPseud)
	if err != nil {
		return nil, err
	}
	xid, err = rs.LastInsertId()
	if err != nil {
		return nil, err
	}
	newPostId := int32(xid)

	// Add the post data.
	_, err = amdb.Exec("INSERT INTO postdata (postid, data) VALUES (?, ?)", newPostId, zeroPost)
	if err != nil {
		return nil, err
	}

	// Add a new topic settings record for the user, too.
	_, err = amdb.Exec("INSERT INTO topicsettings (topicid, uid, last_post) VALUES (?, ?, ?)",
		topic.TopicId, user.Uid, topic.CreateDate)
	if err != nil {
		return nil, err
	}

	amdb.Exec("UNLOCK TABLES;")
	unlock = false

	// TODO: audit record

	return topic, nil
}
