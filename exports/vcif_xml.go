/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
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

	"git.erbosoft.com/amy/amsterdam/database"
)

/*
 * This file defines structures and code for working with Venice Conference Interchange Format (VCIF) files.
 * Amsterdam uses this name for the format for backwards compatibility.
 */

const ISO8601 = "20060102T150405"

// VCIFBase is the top-level element for a VCIF file.
type VCIFBase struct {
	XMLName xml.Name    `xml:"vcif"`  // I am the <vcif> element
	Topics  []VCIFTopic `xml:"topic"` // list of topics
}

// VCIFTopic is the VCIF element representing a topic.
type VCIFTopic struct {
	XMLName  xml.Name   `xml:"topic"`         // I am the <topic> element
	Index    int        `xml:"index,attr"`    // topic index (number, not TopicId)
	Frozen   bool       `xml:"frozen,attr"`   // is topic frozen?
	Archived bool       `xml:"archived,attr"` // is topic archived?
	Name     string     `xml:"topicname"`     // topic name
	Posts    []VCIFPost `xml:"post"`          // posts in the topic
}

// VCIFPost is the VCIF element representing a post in a topic.
type VCIFPost struct {
	XMLName     xml.Name            `xml:"post"`        // I am the <post> element
	ID          int64               `xml:"id,attr"`     // post ID (PostId)
	Parent      int64               `xml:"parent,attr"` // parent PostId (usually 0)
	Index       int                 `xml:"index,attr"`  // post index (number)
	Lines       int                 `xml:"lines,attr"`  // line count
	Author      string              `xml:"author,attr"` // author username
	DateISO8601 string              `xml:"date,attr"`   // post date, ISO 8601 format
	Hidden      bool                `xml:"hidden,attr"` // is post hidden?
	Scribbled   *VCIFScribble       `xml:"scribbled"`   // is post scribbled?
	Pseud       string              `xml:"pseud"`       // post pseud
	Text        string              `xml:"text"`        // post text
	Attachment  *VCIFPostAttachment `xml:"attachment"`  // attachment data
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
	Length     int      `xml:"length,attr"`   // length in bytes
	MIMEType   string   `xml:"type,attr"`     // MIME datatype
	Filename   string   `xml:"filename,attr"` // attachment filename
	Base64Data string   `xml:",chardata"`     // attachment data, Base 64 encoded
}

/* VCIFFromPost fills in a VCIF post structure with data from a conference post.
 * Parameters:
 *     ctx - Standard Go context value.
 *     target - Pointer to the target VCIF post structure.
 *     post - The post to fill the target from.
 * Returns:
 *     Standard Go error status.
 */
func VCIFFromPost(ctx context.Context, target *VCIFPost, post *database.PostHeader) error {
	// Fill in the posting user.
	user, err := post.Creator(ctx)
	if err != nil {
		return err
	}
	target.Author = user.Username

	// Fill in the post text.
	target.Text, err = post.Text(ctx)
	if err != nil {
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
	ainfo, err := post.AttachmentInfo(ctx)
	if err != nil {
		return err
	}
	if ainfo != nil {
		newAttachment := VCIFPostAttachment{
			Length:   int(ainfo.Length),
			MIMEType: ainfo.MIMEType,
			Filename: ainfo.Filename,
		}
		data, err := post.AttachmentData(ctx)
		if err != nil {
			return err
		}
		newAttachment.Base64Data = base64.StdEncoding.EncodeToString(data)
		target.Attachment = &newAttachment
	} else {
		target.Attachment = nil
	}

	// Fill in the rest of the data that can't fail.
	target.ID = post.PostId
	target.Parent = post.Parent
	target.Index = int(post.Num)
	if post.LineCount != nil {
		target.Lines = int(*post.LineCount)
	} else {
		target.Lines = 0
	}
	target.DateISO8601 = post.Posted.Format(ISO8601)
	target.Hidden = post.Hidden
	if post.Pseud != nil {
		target.Pseud = *post.Pseud
	} else {
		target.Pseud = ""
	}
	return nil
}

/* VCIFFromTopic fills in a VCIF topic structure with cata from a conference topic.
 * Parameterrs:
 *     ctx - Standard Go context value.
 *     target - Pointer to the target VCIF topic value.
 *     topic - The topic to fill the target from.
 * Returns:
 *     Standard Go error status.
 */
func VCIFFromTopic(ctx context.Context, target *VCIFTopic, topic *database.Topic) error {
	// Get all posts in the topic.
	posts, err := database.AmGetPostRange(ctx, topic, 0, topic.TopMessage)
	if err != nil {
		return err
	}

	// Build the posts array.
	myPostArray := make([]VCIFPost, len(posts))
	for i, p := range posts {
		err = VCIFFromPost(ctx, &(myPostArray[i]), p)
		if err != nil {
			return fmt.Errorf("error converting post %d: %v", p.Num, err)
		}
	}
	target.Posts = myPostArray

	// Fill in the rest of the data that can't fail.
	target.Index = int(topic.Number)
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
 * Returns:
 *     Standard Go error status.
 */
func VCIFStreamTopicFile(ctx context.Context, w io.Writer, topics []*database.Topic) error {
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
		err = VCIFFromTopic(ctx, &encodedTopic, t)
		if err != nil {
			return fmt.Errorf("error converting topic %d: %v", t.Number, err)
		}
		enc.Encode(encodedTopic)
	}

	// Write the trailing tag.
	_, err = w.Write([]byte("</vcif>\r\n"))
	return err
}
