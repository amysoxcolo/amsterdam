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
// Package exports contains interfacing code for external data formats.
package exports

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
)

/*
 * This file defines structures and code for working with Venice Conference Interchange Format (VCIF) files.
 * Amsterdam uses this name for the format for backwards compatibility.
 */

// ISO8601 is the full ISO 8601 formatting string.
const ISO8601 = "20060102T150405"

// ISO8601_DATE is the ISO 8601 date-only formatting string.
const ISO8601_DATE = "20060102"

// Topic match modes
const (
	VCIFTopicMatchName = 0
	VCIFTopicMatchNum  = 1
)

// VCIFBase is the top-level element for a VCIF file.
type VCIFBase struct {
	XMLName xml.Name    `xml:"vcif"`  // I am the <vcif> element
	Topics  []VCIFTopic `xml:"topic"` // list of topics
}

// VCIFTopic is the VCIF element representing a topic.
type VCIFTopic struct {
	XMLName  xml.Name   `xml:"topic"`         // I am the <topic> element
	Index    int16      `xml:"index,attr"`    // topic index (number, not TopicId)
	Frozen   bool       `xml:"frozen,attr"`   // is topic frozen?
	Archived bool       `xml:"archived,attr"` // is topic archived?
	Name     string     `xml:"topicname"`     // topic name
	Posts    []VCIFPost `xml:"post"`          // posts in the topic
}

// VCIFPost is the VCIF element representing a post in a topic.
type VCIFPost struct {
	XMLName     xml.Name            `xml:"post"`                 // I am the <post> element
	ID          int64               `xml:"id,attr"`              // post ID (PostId)
	Parent      int64               `xml:"parent,attr"`          // parent PostId (usually 0)
	Index       int32               `xml:"index,attr"`           // post index (number)
	Lines       int32               `xml:"lines,attr"`           // line count
	Author      string              `xml:"author,attr"`          // author username
	DateISO8601 string              `xml:"date,attr"`            // post date, ISO 8601 format
	Hidden      bool                `xml:"hidden,attr"`          // is post hidden?
	Scribbled   *VCIFScribble       `xml:"scribbled,omitempty"`  // is post scribbled?
	Pseud       string              `xml:"pseud"`                // post pseud
	Text        string              `xml:"text"`                 // post text
	Attachment  *VCIFPostAttachment `xml:"attachment,omitempty"` // attachment data
}

// VCIFScribble is the VCIF element representing that a post has been scribbled.
type VCIFScribble struct {
	XMLName       xml.Name `xml:"scribbled"` // I am the <scribbled> element
	ByUser        string   `xml:"by,attr"`   // who scribbled it?
	OnDateISO8601 string   `xml:"date,attr"` // scribble date, ISO 8601 format
}

// VCIFPostAttachment is the VCIF element representing an attachment to a post.
type VCIFPostAttachment struct {
	XMLName    xml.Name `xml:"attachment"`    // I am the <attachment> element
	Length     int32    `xml:"length,attr"`   // length in bytes
	MIMEType   string   `xml:"type,attr"`     // MIME datatype
	Filename   string   `xml:"filename,attr"` // attachment filename
	Base64Data string   `xml:",chardata"`     // attachment data, Base 64 encoded
}

/* VCIFFromPost fills in a VCIF post structure with data from a conference post.
 * Parameters:
 *     ctx - Standard Go context value.
 *     target - Pointer to the target VCIF post structure.
 *     post - The post to fill the target from.
 *     bugWorkaround - Work around bug in extracting compressed attachment data.
 * Returns:
 *     Standard Go error status.
 */
func VCIFFromPost(ctx context.Context, target *VCIFPost, post *database.PostHeader, bugWorkaround bool) error {
	// Fill in the posting user.
	user, err := post.Creator(ctx)
	if err != nil {
		return err
	}
	target.Author = user.Username

	// Fill in the post text.
	target.Text, err = post.Text(ctx)
	if err == database.ErrNoPostData {
		target.Text = ""
	} else if err != nil {
		return err
	}

	// Fill in the scribble data.
	if post.IsScribbled() {
		scribbler, err := database.AmGetUser(ctx, *post.ScribbleUid)
		if err != nil {
			return err
		}
		scribbleData := VCIFScribble{
			ByUser:        scribbler.Username,
			OnDateISO8601: post.ScribbleDate.Format(ISO8601),
		}
		target.Scribbled = &scribbleData
	} else {
		target.Scribbled = nil
	}

	// Fill in the attachment data.
	target.Attachment = nil
	if !post.IsScribbled() {
		ainfo, err := post.AttachmentInfo(ctx)
		if err != nil {
			return err
		}
		if ainfo != nil {
			newAttachment := VCIFPostAttachment{
				Length:   ainfo.Length,
				MIMEType: ainfo.MIMEType,
				Filename: ainfo.Filename,
			}
			data, err := post.AttachmentData(ctx, bugWorkaround)
			if err != nil {
				return err
			}
			newAttachment.Base64Data = base64.StdEncoding.EncodeToString(data)
			target.Attachment = &newAttachment
		}
	}

	// Fill in the rest of the data that can't fail.
	target.ID = post.PostId
	target.Parent = post.Parent
	target.Index = post.Num
	if post.LineCount != nil {
		target.Lines = *post.LineCount
	} else {
		target.Lines = 0
	}
	target.DateISO8601 = post.Posted.Format(ISO8601)
	target.Hidden = post.Hidden
	target.Pseud = util.SRef(post.Pseud)
	return nil
}

/* VCIFFromTopic fills in a VCIF topic structure with cata from a conference topic.
 * Parameterrs:
 *     ctx - Standard Go context value.
 *     target - Pointer to the target VCIF topic value.
 *     topic - The topic to fill the target from.
 *     bugWorkaround - Work around bug in extracting compressed attachment data.
 * Returns:
 *     Standard Go error status.
 */
func VCIFFromTopic(ctx context.Context, target *VCIFTopic, topic *database.Topic, bugWorkaround bool) error {
	// Get all posts in the topic.
	posts, err := database.AmGetPostRange(ctx, topic, 0, topic.TopMessage)
	if err != nil {
		return err
	}

	// Build the posts array.
	myPostArray := make([]VCIFPost, len(posts))
	for i, p := range posts {
		err = VCIFFromPost(ctx, &(myPostArray[i]), p, bugWorkaround)
		if err != nil {
			return fmt.Errorf("error converting post %d: %v", p.Num, err)
		}
	}
	target.Posts = myPostArray

	// Fill in the rest of the data that can't fail.
	target.Index = topic.Number
	target.Frozen = topic.Frozen
	target.Archived = topic.Archived
	target.Name = topic.Name
	return nil
}

/* VCIFStreamTopicFile takes a list of topics and writes their VCIF XML encoding to the specified writer.
 * Parameters:
 *     ctx - Standard Go context value.
 *     w - Writer to receive the XML data.
 *     topics - Array of topics to be written.
 *     bugWorkaround - Work around bug in extracting compressed attachment data.
 * Returns:
 *     Standard Go error status.
 */
func VCIFStreamTopicFile(ctx context.Context, w io.Writer, topics []*database.Topic, bugWorkaround bool) error {
	// Write the header of the file.
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString("\r\n<vcif>\r\n")
	_, err := w.Write([]byte(b.String()))
	if err != nil {
		return err
	}

	// Create the XML encoder.
	enc := xml.NewEncoder(w)
	enc.Indent("  ", "  ")

	// Encode each topic in turn and write it to the writer.
	for _, t := range topics {
		var encodedTopic VCIFTopic
		err = VCIFFromTopic(ctx, &encodedTopic, t, bugWorkaround)
		if err != nil {
			return fmt.Errorf("error converting topic %d: %v", t.Number, err)
		}
		enc.Encode(encodedTopic)
	}

	// Write the trailing tag.
	_, err = w.Write([]byte("</vcif>\r\n"))
	return err
}

// augmentPost sets the auxiliary data on a post that's not set by just posting it.
func augmentPost(ctx context.Context, post *database.PostHeader, pdata *VCIFPost, comm *database.Community, loader *database.User, ipaddr string) error {
	dateStamp, err := time.Parse(ISO8601, pdata.DateISO8601)
	if err != nil {
		dateStamp = time.Now().UTC()
	}
	err = post.ImportFix(ctx, pdata.Parent, dateStamp)
	if err != nil {
		return err
	}
	if pdata.Scribbled != nil {
		creator, _ := post.Creator(ctx)
		err = post.Scribble(ctx, creator, comm, ipaddr)
		if err != nil {
			return err
		}
	} else if pdata.Attachment != nil {
		data, err := base64.StdEncoding.DecodeString(pdata.Attachment.Base64Data)
		if err != nil {
			return err
		}
		creator, _ := post.Creator(ctx)
		err = post.SetAttachment(ctx, creator, pdata.Attachment.Filename, pdata.Attachment.MIMEType, pdata.Attachment.Length, data, comm, ipaddr)
		if err != nil {
			return err
		}
	}
	if pdata.Hidden && pdata.Scribbled == nil {
		err = post.SetHidden(ctx, loader, true, comm, ipaddr)
		if err != nil {
			return err
		}
	}
	return nil
}

func matchOrCreateTopic(ctx context.Context, comm *database.Community, conf *database.Conference, tdata *VCIFTopic, matchMode int, createNew bool, loader *database.User,
	ipaddr string) (*database.Topic, bool, []string, error) {
	scroll := make([]string, 0)
	var topic *database.Topic
	var err error
	switch matchMode {
	case VCIFTopicMatchName:
		topic, err = database.AmGetTopicByName(ctx, conf, tdata.Name)
	case VCIFTopicMatchNum:
		topic, err = database.AmGetTopicByNumber(ctx, conf, tdata.Index)
	}
	if err == nil {
		return topic, false, scroll, nil
	} else if err != database.ErrNoTopic {
		return nil, false, scroll, err
	}
	if !createNew {
		return nil, false, scroll, database.ErrNoTopic
	}
	scroll = append(scroll, fmt.Sprintf("Topic \"%s\" doesn't exist...creating...", tdata.Name))
	zeroPost := &(tdata.Posts[0])
	author, err := database.AmGetUserByName(ctx, zeroPost.Author, nil)
	if err != nil {
		return nil, true, scroll, err
	}
	topic, err = database.AmNewTopic(ctx, conf, author, tdata.Name, zeroPost.Pseud, zeroPost.Text, zeroPost.Lines, comm, ipaddr)
	if err != nil {
		return nil, true, scroll, err
	}
	zp, err := topic.GetPost(ctx, 0)
	if err != nil {
		return nil, true, scroll, err
	}
	err = augmentPost(ctx, zp, zeroPost, comm, loader, ipaddr)
	if err != nil {
		return nil, true, scroll, err
	}
	return topic, true, scroll, nil
}

func VCIFImportMessages(ctx context.Context, r io.Reader, comm *database.Community, conf *database.Conference, matchMode int, createNew bool, loader *database.User,
	ipaddr string) (int, int, []string, error) {
	dec := xml.NewDecoder(r)
	var importData VCIFBase
	err := dec.Decode(&importData)
	if err != nil {
		return 0, 0, make([]string, 0), err
	}

	scroll := make([]string, 0)
	topicCount := 0
	postCount := 0
	for _, tdata := range importData.Topics {
		topic, created, subscroll, err := matchOrCreateTopic(ctx, comm, conf, &tdata, matchMode, createNew, loader, ipaddr)
		scroll = append(scroll, subscroll...)
		if err != nil {
			if created {
				scroll = append(scroll, fmt.Sprintf("Unable to create topic \"%s\": %v", tdata.Name, err))
			} else {
				scroll = append(scroll, fmt.Sprintf("Unable to locate topic \"%s\": %v", tdata.Name, err))
			}
		} else {
			topicCount++
			if created {
				scroll = append(scroll, fmt.Sprintf("New topic \"%s\" created with number %d", tdata.Name, topic.Number))
			}
			// If we created the topic, the "zero post" was already posted, so skip it in this loop so we don't post it again.
			skipPost := created
			topicOK := 0
			topicFail := 0
			for _, pdata := range tdata.Posts {
				if skipPost {
					skipPost = false
					continue
				}
				author, err := database.AmGetUserByName(ctx, pdata.Author, nil)
				if err != nil {
					author = loader
				}
				post, err := database.AmNewPost(ctx, conf, topic, author, pdata.Pseud, pdata.Text, pdata.Lines, comm, ipaddr)
				if err == nil {
					err = augmentPost(ctx, post, &pdata, comm, loader, ipaddr)
				}
				if err == nil {
					topicOK++
					postCount++
				} else {
					topicFail++
					scroll = append(scroll, fmt.Sprintf("Unable to post message %d to topic \"%s\": %v", pdata.Index, topic.Name, err))
				}
			}
			scroll = append(scroll, fmt.Sprintf("In topic \"%s\": %d messages posted successfully, %d failed", topic.Name, topicOK, topicFail))
			if created {
				if tdata.Frozen {
					err = topic.SetFrozen(ctx, true, loader, comm, ipaddr)
					if err != nil {
						scroll = append(scroll, fmt.Sprintf("Unable to freeze topic \"%s\": %v", topic.Name, err))
					}
				}
				if tdata.Archived {
					err = topic.SetArchived(ctx, true, loader, comm, ipaddr)
					if err != nil {
						scroll = append(scroll, fmt.Sprintf("Unable to archive topic \"%s\": %v", topic.Name, err))
					}
				}
			}
		}
	}
	return topicCount, postCount, scroll, nil
}
