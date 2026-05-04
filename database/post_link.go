/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */
// The database package contains database management and storage logic.
package database

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Post link scopes.
const (
	PLSCOPE_GLOBAL     = "global"
	PLSCOPE_COMMUNITY  = "community"
	PLSCOPE_CONFERENCE = "conference"
	PLSCOPE_TOPIC      = "topic"
	PLSCOPE_ERROR      = "error"
)

// Post link classifications.
const (
	PLCLASS_COMMUNITY     = "community"
	PLCLASS_CONFERENCE    = "conference"
	PLCLASS_TOPIC         = "topic"
	PLCLASS_POST          = "post"
	PLCLASS_POSTRANGE     = "postrange"
	PLCLASS_POSTOPENRANGE = "postopenrange"
)

// PostLinkData is the structure holding the decoded parts of the post link.
type PostLinkData struct {
	Community  string
	CommId     int32
	Conference string
	Topic      int16
	FirstPost  int32
	LastPost   int32
}

// NeedsDBVerification returns true if the post link data needs tro be varified against the database.
func (d *PostLinkData) NeedsDBVerification() bool {
	return d.Community != "" || d.Conference != ""
}

// VerifyNames verifies the post link data against the database.
func (d *PostLinkData) VerifyNames(ctx context.Context) error {
	commid := d.CommId
	if d.Community != "" {
		comm, err := AmGetCommunityByAlias(ctx, d.Community)
		if err != nil {
			return err
		}
		if comm == nil {
			return errors.New("community alias not found")
		}
		commid = comm.Id
	}
	if d.Conference != "" {
		conf, err := AmGetConferenceByAlias(ctx, commid, d.Conference)
		if err != nil {
			return err
		}
		if conf == nil {
			return errors.New("conference alias not found")
		}
	}
	return nil
}

// AsString converts the post link data to a string reference.
func (d *PostLinkData) AsString() string {
	var b strings.Builder
	if d.Community != "" {
		b.WriteString(d.Community)
		b.WriteString("!")
	}
	wrote := false
	if d.Conference != "" {
		b.WriteString(d.Conference)
		b.WriteString(".")
		wrote = true
	}
	needDot := false
	if d.Topic > 0 {
		needDot = true
		b.WriteString(fmt.Sprintf("%d", d.Topic))
		if !wrote {
			b.WriteString(".")
			needDot = false
		}
	}
	if d.FirstPost >= 0 {
		s := ""
		if d.LastPost < 0 || d.LastPost == d.FirstPost {
			s = fmt.Sprintf("%d", d.FirstPost)
		} else {
			s = fmt.Sprintf("%d-%d", d.FirstPost, d.LastPost)
		}
		if needDot {
			b.WriteString(".")
		}
		b.WriteString(s)
	}
	return b.String()
}

/* Classify tells us what kind of post link this is and where we should interpret it from.
 * Returns:
 *     String value indicating the scope of the link:
 *         "global" - Scope across the entire site.
 *         "community" - Scope within a community.
 *         "conference" - Scope within a conference.
 *         "topic" - Scope within a specific topic.
 *         Empty string - Null link.
 *     String value indicating what the link points to:
 *         "community" - Points to a community.
 *         "conference" - Points to a conference.
 *         "topic" - Points to a topic.
 *         "post" - Points to a single post within the topic.
 *         "postrange" - Points to a range of posts within the topic.
 *         "postopenrange" - Points to an open-ended range of posts within the topic.
 *         Empty string - Null link.
 */
func (d *PostLinkData) Classify() (string, string) {
	if d.Community == "" {
		if d.Conference == "" {
			if d.Topic == -1 {
				if d.FirstPost == -1 {
					return "", ""
				} else if d.LastPost == -1 {
					return PLSCOPE_TOPIC, PLCLASS_POSTOPENRANGE
				} else if d.LastPost == d.FirstPost {
					return PLSCOPE_TOPIC, PLCLASS_POST
				} else {
					return PLSCOPE_TOPIC, PLCLASS_POSTRANGE
				}
			} else {
				if d.FirstPost == -1 {
					return PLSCOPE_CONFERENCE, PLCLASS_TOPIC
				} else if d.LastPost == -1 {
					return PLSCOPE_CONFERENCE, PLCLASS_POSTOPENRANGE
				} else if d.LastPost == d.FirstPost {
					return PLSCOPE_CONFERENCE, PLCLASS_POST
				} else {
					return PLSCOPE_CONFERENCE, PLCLASS_POSTRANGE
				}
			}
		} else {
			if d.Topic == -1 {
				return PLSCOPE_COMMUNITY, PLCLASS_CONFERENCE
			} else {
				if d.FirstPost == -1 {
					return PLSCOPE_COMMUNITY, PLCLASS_TOPIC
				} else if d.LastPost == -1 {
					return PLSCOPE_COMMUNITY, PLCLASS_POSTOPENRANGE
				} else if d.LastPost == d.FirstPost {
					return PLSCOPE_COMMUNITY, PLCLASS_POST
				} else {
					return PLSCOPE_COMMUNITY, PLCLASS_POSTRANGE
				}
			}
		}
	} else {
		if d.Conference == "" {
			return PLSCOPE_GLOBAL, PLCLASS_COMMUNITY
		} else {
			if d.Topic == -1 {
				return PLSCOPE_GLOBAL, PLCLASS_CONFERENCE
			} else {
				if d.FirstPost == -1 {
					return PLSCOPE_GLOBAL, PLCLASS_TOPIC
				} else if d.LastPost == -1 {
					return PLSCOPE_GLOBAL, PLCLASS_POSTOPENRANGE
				} else if d.LastPost == d.FirstPost {
					return PLSCOPE_GLOBAL, PLCLASS_POST
				} else {
					return PLSCOPE_GLOBAL, PLCLASS_POSTRANGE
				}
			}
		}
	}
}

// Maximum lengths of the components.
const (
	maxLinkLength       = 130
	maxCommunityLength  = 32
	maxConferenceLength = 64
)

// validateCommunity validates the community name and saves it.
func validateCommunity(name string, rc *PostLinkData) error {
	if len(name) > maxCommunityLength {
		return errors.New("community alias is too long")
	}
	if !AmIsValidAmsterdamID(name) {
		return errors.New("community alias is not a valid identifier")
	}
	rc.Community = name
	return nil
}

// validateConference validates the conference name and saves it.
func validateConference(name string, rc *PostLinkData) error {
	if len(name) > maxConferenceLength {
		return errors.New("conference alias is too long")
	}
	if !AmIsValidAmsterdamID(name) {
		return errors.New("conference alias is not a valid identifier")
	}
	rc.Conference = name
	return nil
}

// decodeTopicNumber decodes the topic number and saves it.
func decodeTopicNumber(data string, rc *PostLinkData) error {
	v, err := strconv.Atoi(data)
	if err != nil {
		return errors.New("invalid topic number reference")
	}
	if v > math.MaxInt16 {
		return errors.New("topic number out of range")
	}
	rc.Topic = int16(v)
	return nil
}

// decodePostRange decodes the post ranges (first and last post) and saves them.
func decodePostRange(data string, rc *PostLinkData) error {
	pos := strings.IndexByte(data, '-')
	var tempVal int32 = -1
	if pos > 0 {
		temp := data[:pos]
		data = data[pos+1:]
		v, err := strconv.Atoi(temp)
		if err != nil {
			return errors.New("invalid post number reference")
		}
		tempVal = int32(v)

		if len(data) == 0 {
			// range is open-ended (number-)
			rc.FirstPost = tempVal
			rc.LastPost = -1
			return nil
		}
	} else if pos == 0 {
		return errors.New("cannot have - at beginning of post range")
	}

	v2, err := strconv.Atoi(data)
	if err != nil {
		return errors.New("invalid post number reference")
	}
	rc.FirstPost = int32(v2)
	if tempVal >= 0 {
		if tempVal < rc.FirstPost {
			// "frontwards" range - reorder the components
			rc.LastPost = rc.FirstPost
			rc.FirstPost = tempVal
		} else {
			// "backwards" range
			rc.LastPost = tempVal
		}
	} else {
		// a "range" of a single post
		rc.LastPost = rc.FirstPost
	}
	return nil
}

/* AmDecodePostLink decodes a post link and returns the complete breakdown of its components.
 * Parameters:
 *     data - The post link to be decoded.
 * Returns:
 *     Pointer to structure containing post link data, or nil.
 *     Standard Go error status.
 */
func AmDecodePostLink(data string) (*PostLinkData, error) {
	if data == "" {
		return nil, errors.New("empty string")
	}
	if len(data) > maxLinkLength {
		return nil, errors.New("post link string too long")
	}
	rc := new(PostLinkData{
		Community:  "",
		Conference: "",
		Topic:      -1,
		FirstPost:  -1,
		LastPost:   -1,
	})

	work := data
	// First test: Bang
	pos := strings.IndexByte(work, '!')
	if pos > 0 {
		err := validateCommunity(work[:pos], rc)
		if err != nil {
			return nil, err
		}
		work = work[pos+1:]
		if len(work) == 0 {
			return rc, nil // community link
		}
	} else if pos == 0 {
		return nil, errors.New("cannot have ! at beginning")
	}

	// Second test: Dot #1
	pos = strings.IndexByte(work, '.')
	if pos < 0 {
		// no dots in here, must be either "postlink" or "community!conference"
		var err error
		if rc.Community == "" {
			err = decodePostRange(work, rc)
		} else {
			err = validateConference(work, rc)
		}
		if err != nil {
			return nil, err
		}
		return rc, nil
	}

	// Peel off the initial substring before the dot.
	confOrTopic := work[:pos]
	work = work[pos+1:]
	if len(work) == 0 {
		// we had "conference." or "topic." or maybe "community!conference."
		var err error
		if rc.Community == "" {
			// it's either "conference." or "topic." - try the latter first
			err = decodeTopicNumber(confOrTopic, rc)
			if err != nil {
				// it's not a topic number, try it as a conference name
				err = validateConference(confOrTopic, rc)
			}
		} else {
			// it was "community!conference."
			err = validateConference(confOrTopic, rc)
		}
		if err != nil {
			return nil, err
		}
		return rc, nil
	}

	// Third test: Dot #2
	pos = strings.IndexByte(work, '.')
	if pos < 0 {
		// we had "conference.topic" or "topic.posts" or maybe "community!conference.topic"
		var err error
		if rc.Community == "" {
			// either "conference.topic" or "topic.posts"
			isTopic := false
			err = decodeTopicNumber(confOrTopic, rc)
			if err != nil {
				// it's "conference.topic"
				err = validateConference(confOrTopic, rc)
				isTopic = true
			}
			if err == nil {
				if isTopic {
					err = decodeTopicNumber(work, rc)
				} else {
					err = decodePostRange(work, rc)
				}
			}
		} else {
			// we have "community!conference.topic"
			err = validateConference(confOrTopic, rc)
			if err == nil {
				err = decodeTopicNumber(work, rc)
			}
		}
		if err != nil {
			return nil, err
		}
		return rc, nil
	} else if pos == 0 {
		return nil, errors.New("cannot have . at beginning of string")
	}

	// We definitely have "conference.topic.something" or "community!conference.topic.something"
	err := validateConference(confOrTopic, rc)
	if err == nil {
		err = decodeTopicNumber(work[:pos], rc)
	}
	if err != nil {
		return nil, err
	}
	work = work[pos+1:]
	if len(work) == 0 {
		// we had "conference.topic." or "communtiy!conference.topic.", those are both valid
		return rc, nil
	}
	err = decodePostRange(work, rc) // the rest must be the post range
	if err != nil {
		return nil, err
	}
	return rc, nil
}

// AmCreatePostLinkContext creates a new empty post link context.
func AmCreatePostLinkContext(community string, commid int32, conference string, topic int16) *PostLinkData {
	return new(PostLinkData{
		Community:  community,
		CommId:     commid,
		Conference: conference,
		Topic:      topic,
		FirstPost:  -1,
		LastPost:   -1,
	})
}
