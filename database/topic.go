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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// Topic is the top-level structure detailing topics.
type Topic struct {
	TopicId    int32     `db:"topicid"`     // unique ID of the topic
	ConfId     int32     `db:"confid"`      // conference this topic is in
	Number     int16     `db:"num"`         // topic number
	CreatorUid int32     `db:"creator_uid"` // UID of topic creator
	TopMessage int32     `db:"top_message"` // highest message number in topic
	Frozen     bool      `db:"frozen"`      // frozen topic
	Archived   bool      `db:"archived"`    // archived topic
	Sticky     bool      `db:"sticky"`      // sticky topic
	CreateDate time.Time `db:"createdate"`  // creation date
	LastUpdate time.Time `db:"lastupdate"`  // last update date
	Name       string    `db:"name"`        // topic name
}

// GetPost returns a post in the topic by number.
func (t *Topic) GetPost(ctx context.Context, num int32) (*PostHeader, error) {
	if num > t.TopMessage {
		return nil, fmt.Errorf("no post %d in topic %d", num, t.TopicId)
	}
	var dbdata []PostHeader
	err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM posts WHERE topicid = ? AND num = ?", t.TopicId, num)
	if err == nil {
		if len(dbdata) == 0 {
			err = fmt.Errorf("no post %d in topic %d", num, t.TopicId)
		} else if len(dbdata) > 1 {
			err = fmt.Errorf("topic.GetPost: too many entries (%d) for post %d in topic %d", len(dbdata), num, t.TopicId)
		} else {
			return &(dbdata[0]), nil
		}
	}
	return nil, err
}

// GetLastRead returns the "last read" message for a user on a topic.
func (t *Topic) GetLastRead(ctx context.Context, u *User) (int32, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT last_message FROM topicsettings WHERE topicid = ? AND uid = ?", t.TopicId, u.Uid)
	if err != nil {
		return -1, err
	}
	var rc int32 = -1
	if rs.Next() {
		err = rs.Scan(&rc)
	}
	return rc, err
}

// SetLastRead sets the "last read" message for a user on a topic.
func (t *Topic) SetLastRead(ctx context.Context, u *User, postNum int32) error {
	rs, err := amdb.ExecContext(ctx, "UPDATE topicsettings SET last_message = ?, last_read = NOW() WHERE topicid = ? AND uid = ?",
		postNum, t.TopicId, u.Uid)
	if err == nil {
		nrow, _ := rs.RowsAffected()
		if nrow == 0 {
			_, err = amdb.ExecContext(ctx, "INSERT INTO topicsettings (topicid, uid, last_message, last_read, last_post) VALUES (?, ?, ?, NOW(), NULL)",
				t.TopicId, u.Uid, postNum)
		}
	}
	return err
}

// IsHidden tells us whether the user has the topic hidden.
func (t *Topic) IsHidden(ctx context.Context, u *User) (bool, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT hidden FROM topicsettings WHERE topicid = ? AND uid = ?", t.TopicId, u.Uid)
	if err != nil {
		return false, err
	}
	rc := false
	if rs.Next() {
		err = rs.Scan(&rc)
	}
	return rc, err
}

// SetHidden sets the "hidden" state on a topic for a user.
func (t *Topic) SetHidden(ctx context.Context, u *User, hidden bool) error {
	rs, err := amdb.ExecContext(ctx, "UPDATE topicsettings SET hidden = ? WHERE topicid = ? AND uid = ?", hidden, t.TopicId, u.Uid)
	if err == nil {
		nrow, _ := rs.RowsAffected()
		if nrow == 0 {
			_, err = amdb.ExecContext(ctx, "INSERT INTO topicsettings (topicid, uid, hidden) VALUES (?, ?, ?)", t.TopicId, u.Uid, hidden)
		}
	}
	return err
}

// TopicSettings contains per-user settings for topics, including the "last read" message pointer.
type TopicSettings struct {
	TopicId     int32      `db:"topicid"`      // unique ID of the topic
	Uid         int32      `db:"uid"`          // UID of the user
	Hidden      bool       `db:"hidden"`       // has user hidden topic?
	LastMessage int32      `db:"last_message"` // last message read
	LastRead    *time.Time `db:"last_read"`    // time of last read
	LastPost    *time.Time `db:"last_post"`    // time of last post
	Subscribe   bool       `db:"subscribe"`    // subscribed to topic updates?
}

// TopicSummary is a smaller data structure that gets topic information to create the topic list display.
type TopicSummary struct {
	TopicID    int32     // the topic ID
	Number     int16     // the number of the topic
	Name       string    // the topic name
	Unread     int32     // number of unread messages
	Total      int32     // total number of messages
	LastUpdate time.Time // last update timestamp
	Frozen     bool      // is topic frozen?
	Archived   bool      // is topic archived?
	Subscribed bool      // is topic subscribed?
	Hidden     bool      // is topic hidden?
	Sticky     bool      // is topic sticky?
	NewFlag    bool      // does topic have new messages?
}

/* AmGetTopic retrieves a topic by ID.
 * Parameters:
 *     ctx - Standard Go context value.
 *     topicId - ID of the topic to retrieve.
 * Returns:
 *     The topic pointer, or nil.
 *     Standard Go error status.
 */
func AmGetTopic(ctx context.Context, topicId int32) (*Topic, error) {
	var dbdata []Topic
	if err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM topics WHERE topicid = ?", topicId); err != nil {
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

/* AmGetTopicTx retrieves a topic by ID, in a transaction.
 * Parameters:
 *     ctx - Standard Go context value.
 *     tx - The transaction to use.
 *     topicId - ID of the topic to retrieve.
 * Returns:
 *     The topic pointer, or nil.
 *     Standard Go error status.
 */
func AmGetTopicTx(ctx context.Context, tx *sqlx.Tx, topicId int32) (*Topic, error) {
	var dbdata []Topic
	if err := tx.SelectContext(ctx, &dbdata, "SELECT * FROM topics WHERE topicid = ?", topicId); err != nil {
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

/* AmGetTopicByNumber retrieves a topic by conference and sequence number.
 * Parameters:
 *     ctx - Standard Go context value.
 *     conf - The conference to look in.
 *     topicNum - The topic number within that conference.
 * Returns:
 *     Pointer to the Topic, or nil.
 *     Standard Go error status.
 */
func AmGetTopicByNumber(ctx context.Context, conf *Conference, topicNum int16) (*Topic, error) {
	var dbdata []Topic
	err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM topics WHERE confid = ? AND num = ?", conf.ConfId, topicNum)
	if err == nil {
		if len(dbdata) == 0 {
			err = fmt.Errorf("no topic numbered %d in conference %s (#%d)", topicNum, conf.Name, conf.ConfId)
		} else if len(dbdata) > 1 {
			err = fmt.Errorf("AmGetTopicByNumber: too many entries (%d) for topic #%d in conference %s (#%d)", len(dbdata), topicNum, conf.Name, conf.ConfId)
		} else {
			return &(dbdata[0]), nil
		}
	}
	return nil, err
}

// View and sort constants for AmListTopics.
const (
	TopicViewAll        = 0 // list all topics
	TopicViewNew        = 1 // list only visible topics with new messages
	TopicViewActive     = 2 // list only visible topics, active first
	TopicViewAllVisible = 3 // list only visible topics
	TopicViewHidden     = 4 // list only hidden topics
	TopicViewArchive    = 5 // list only archived, non-hidden topics

	TopicSortID     = 0 // sort by topic ID
	TopicSortNumber = 1 // sort by topic number
	TopicSortName   = 2 // sort by name
	TopicSortUnread = 3 // sort by number of unread messages
	TopicSortTotal  = 4 // sort by total number of messages
	TopicSortDate   = 5 // sort by date of last update
)

/* AmListTopics produces a list of topic summary information according to specific options.
 * Parameters:
 *     ctx - Standard Go context value.
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
func AmListTopics(ctx context.Context, confid int32, uid int32, viewOption int, sortOption int, ignoreSticky bool) ([]*TopicSummary, error) {
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
		whereClause = "t.archived = 0 AND (s.hidden IS NULL OR s.hidden = 0) AND " + tail
	case TopicViewActive:
		whereClause = "t.archived = 0 AND (s.hidden IS NULL OR s.hidden = 0)"
	case TopicViewAllVisible:
		whereClause = "(s.hidden IS NULL OR s.hidden = 0)"
	case TopicViewHidden:
		whereClause = "s.hidden = 1"
	case TopicViewArchive:
		whereClause = "t.archived = 1 AND (s.hidden IS NULL OR s.hidden = 0)"
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
	fullStatement.WriteString("IFNULL(s.hidden,0) AS hidden, t.sticky, GREATEST(SIGN(t.top_message - IFNULL(s.last_message,-1)),0) AS newflag ")
	fullStatement.WriteString("FROM topics t LEFT JOIN topicsettings s ON t.topicid = s.topicid AND s.uid = ? WHERE t.confid = ? ")
	if whereClause != "" {
		fullStatement.WriteString("AND ")
		fullStatement.WriteString(whereClause)
	}
	fullStatement.WriteString(" ORDER BY ")
	if !ignoreSticky {
		fullStatement.WriteString("t.sticky DESC, ")
	}
	if viewOption == TopicViewActive {
		fullStatement.WriteString("newflag DESC, ")
	}
	fullStatement.WriteString(orderByClause)

	// Execute and capture results
	rs, err := amdb.QueryContext(ctx, fullStatement.String(), uid, confid)
	if err != nil {
		return nil, err
	}
	rc := make([]*TopicSummary, 0)
	for rs.Next() {
		var rec TopicSummary
		if err = rs.Scan(&rec.TopicID, &rec.Number, &rec.Name, &rec.Unread, &rec.Total, &rec.LastUpdate, &rec.Frozen,
			&rec.Archived, &rec.Subscribed, &rec.Hidden, &rec.Sticky, &rec.NewFlag); err != nil {
			log.Errorf("AmListTopics scan error: %v", err)
		} else {
			rc = append(rc, &rec)
		}
	}
	return rc, nil
}

/* AmNewTopic creates a new topic.
 * Parameters:
 *     ctx - Standard Go context value.
 *     conf - Conference to add the new post.
 *     user - User creating the new topic.
 *     title - The new topic's title.
 *     zeroPostPseud - Pseud for the topic's "zero post" (first post).
 *     zeroPost - Textual data for the zero post.
 *     zeroPostLines - Number of lines of text in zeroPost.
 *     ipaddr - IP address of the user making the topic, for audit purposes.
 * Returns:
 *     Pointer to the new Topic data structure.
 *     Standard Go error status.
 */
func AmNewTopic(ctx context.Context, conf *Conference, user *User, title string, zeroPostPseud string, zeroPost string,
	zeroPostLines int32, ipaddr string) (*Topic, error) {
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

	unlock := true
	tx.ExecContext(ctx, "LOCK TABLES confs WRITE, topics WRITE, topicsettings WRITE, posts WRITE, postdata WRITE;")
	defer func() {
		if unlock {
			tx.ExecContext(ctx, "UNLOCK TABLES;")
		}
	}()

	// Insert the new topic into the database.
	conf.Mutex.Lock()
	rs, err := tx.ExecContext(ctx, "INSERT INTO topics (confid, num, creator_uid, createdate, lastupdate, name) VALUES (?, ?, ?, NOW(), NOW(), ?)",
		conf.ConfId, conf.TopTopic+1, user.Uid, title)
	if err != nil {
		conf.Mutex.Unlock()
		return nil, err
	}
	// Retrieve the ID of the new topic.
	xid, err := rs.LastInsertId()
	if err != nil {
		conf.Mutex.Unlock()
		return nil, err
	}
	// Get the topic.
	topic, err := AmGetTopicTx(ctx, tx, int32(xid))
	if err != nil {
		conf.Mutex.Unlock()
		return nil, err
	}

	// Update the conference to set the last update and top topic.
	_, err = tx.ExecContext(ctx, "UPDATE confs SET lastupdate = ?, top_topic = ? WHERE confid = ?", topic.CreateDate, conf.TopTopic+1, conf.ConfId)
	if err != nil {
		conf.Mutex.Unlock()
		return nil, err
	}
	conf.TopTopic++
	conf.LastUpdate = &topic.CreateDate
	conf.Mutex.Unlock()

	// Add the "header record" for the first post.
	rs, err = tx.ExecContext(ctx, "INSERT INTO posts (topicid, num, linecount, creator_uid, posted, pseud) VALUES (?, 0, ?, ?, ?, ?)",
		topic.TopicId, zeroPostLines, user.Uid, topic.CreateDate, zeroPostPseud)
	if err != nil {
		return nil, err
	}
	xid, err = rs.LastInsertId()
	if err != nil {
		return nil, err
	}
	// Add the post data.
	_, err = tx.ExecContext(ctx, "INSERT INTO postdata (postid, data) VALUES (?, ?)", int32(xid), zeroPost)
	if err != nil {
		return nil, err
	}

	// Add a new topic settings record for the user, too.
	_, err = tx.ExecContext(ctx, "INSERT INTO topicsettings (topicid, uid, last_post) VALUES (?, ?, ?)",
		topic.TopicId, user.Uid, topic.CreateDate)
	if err != nil {
		return nil, err
	}

	tx.ExecContext(ctx, "UNLOCK TABLES;")
	unlock = false

	// update the "last posted" date in the conference settings
	_, err = conf.TouchPost(ctx, tx, user, topic.CreateDate)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	success = true

	// create audit record
	ar = AmNewAudit(AuditConferenceCreateTopic, user.Uid, ipaddr, fmt.Sprintf("confid=%d", conf.ConfId),
		fmt.Sprintf("num=%d", topic.Number), fmt.Sprintf("name=%s", topic.Name))

	return topic, nil
}
