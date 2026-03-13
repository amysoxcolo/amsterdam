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

// Package email contains support for E-mail messages sent by Amsterdam.
package email

import (
	"bytes"
	"context"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/htmlcheck"
	"github.com/CloudyKit/jet/v6"
	log "github.com/sirupsen/logrus"
)

/* AmDeliverSubscription takes a message that's just been posted to a topic and sends it to the list of subscribers
 * to that topic. It's intended to be executed in a worker pool task.
 * Parameters:
 *     ctx - Standard Go context parameter.
 *     comm - Community the message was posted to.
 *     conf - Conference the message was posted to.
 *     confAlias - Current alias for that conference.
 *     topic - Current topic the message was posted to.
 *     poster - User that posted the message.
 *     post - The message that was posted.
 *     text - The text of that post.
 *     recipUids - Array of user IDs representing recipients of E-mail messages.
 *     ipaddr - IP address of the poster, used for mail tracing.
 */
func AmDeliverSubscription(ctx context.Context, comm *database.Community, conf *database.Conference, confAlias string,
	topic *database.Topic, poster *database.User, post *database.PostHeader, text string, recipUids []int32, ipaddr string) {
	log.Debugf("AmDeliverSubscription kicked off by %s with %d mail(s) to deliver", poster.Username, len(recipUids))

	// Preprocess the text and the pseud.
	checker, err := htmlcheck.AmNewHTMLChecker(ctx, "mail-post")
	if err != nil {
		log.Errorf("AmDeliverSubscription: failed to get HTML checker (%v)", err)
		return
	}
	var realText string
	err = checker.Append(text)
	if err == nil {
		err = checker.Finish()
		if err == nil {
			realText, err = checker.Value()
		}
	}
	if err != nil {
		log.Errorf("AmDeliverSubscription: failed to process post text (%v)", err)
		return
	}
	checker.Reset()
	var realPseud string
	if post.Pseud != nil {
		err = checker.Append(*post.Pseud)
		if err == nil {
			err = checker.Finish()
			if err == nil {
				realPseud, err = checker.Value()
			}
		}
	} else {
		realPseud = ""
	}
	if err != nil {
		log.Errorf("AmDeliverSubscription: failed to process post pseud (%v)", err)
		return
	}

	// Use Jet to format the message directly. We bypass the regular formatter in formatMessage in sender.go because
	// we don't want to have to format the message once per recipient.
	templ, err := emailRenderer.GetTemplate("mailpost.jet")
	if err != nil {
		log.Errorf("AmDeliverSubscription: failed to load template (%v)", err)
		return
	}
	vars := make(jet.VarMap)
	vars.Set("userName", poster.Username)
	vars.Set("communityName", comm.Name)
	vars.Set("conferenceName", conf.Name)
	vars.Set("topicName", topic.Name)
	pl := database.AmCreatePostLinkContext(comm.Alias, confAlias, topic.Number)
	vars.Set("topicLink", pl.AsString())
	vars.Set("pseud", realPseud)
	vars.Set("text", realText)
	subjectSink := AmNewEmailMessage(0, "")
	var textBuf bytes.Buffer
	err = templ.Execute(&textBuf, vars, subjectSink)
	if err != nil {
		log.Errorf("AmDeliverSubscription: failed to format template (%v)", err)
		return
	}
	sendText := textBuf.String()

	// The delivery loop; build each message and send it.  Note that sending a message puts the Message structure on
	// the sender goroutine channel, so we have to create a new Message each time, unlike what we did in Venice.
SendLoop:
	for i := range recipUids {
		select {
		case <-ctx.Done():
			log.Errorf("AmDeliverSubscription: aborted on send loop iter %d with %v", i+1, ctx.Err())
			break SendLoop
		default:
			if ci, err := database.AmGetContactInfoForUser(ctx, recipUids[i]); err == nil {
				msg := AmNewEmailMessage(poster.Uid, ipaddr)
				msg.SetSubject(subjectSink.GetSubject())
				msg.SetText(sendText)
				msg.AddTo(*ci.Email, ci.FullName(false))
				msg.Send()
			} else {
				log.Warnf("AmDeliverSubscription skipped uid %d because no contact info retrieved (%v)", recipUids[i], err)
			}
		}
	}
}
