/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package main contains the high-level Amsterdam logic.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
)

// slurpFile reads the contrents of a multipart.File into memory.
func slurpFile(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

/* AttachmentUpload adds an attachment to a post.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func AttachmentUpload(ctxt ui.AmContext) (string, any, error) {
	target := ctxt.FormField("tgt")
	postidStr := ctxt.FormField("post")
	postId, err := strconv.ParseInt(postidStr, 10, 64)
	if err != nil {
		return ui.ErrorPage(ctxt, fmt.Errorf("internal error converting postID: %v", err))
	}
	if ctxt.FormFieldIsSet("upload") {
		file, err := ctxt.FormFile("thefile")
		if err == nil {
			var post *database.PostHeader
			post, err = database.AmGetPost(ctxt.Ctx(), postId)
			if err == nil {
				var data []byte
				data, err = slurpFile(file)
				if err == nil {
					err = post.SetAttachment(ctxt.Ctx(), ctxt.CurrentUser(), file.Filename, file.Header.Get("Content-Type"), int32(file.Size), data, ctxt.RemoteIP())
					if err == nil {
						return "redirect", target, nil
					}
				}
			}
		}

		ctxt.VarMap().Set("target", target)
		ctxt.VarMap().Set("post", postId)
		ctxt.VarMap().Set("amsterdam_pageTitle", "Upload Attachment")
		ctxt.VarMap().Set("errorMessage", err.Error())
		return "framed_template", "attachment_upload.jet", nil
	}
	return ui.ErrorPage(ctxt, errors.New("invalid button clicked on form"))
}

/* AttachmentSend sends the data of an attachment to the browser.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func AttachmentSend(ctxt ui.AmContext) (string, any, error) {
	postIdStr := ctxt.URLParam("post")
	postId, err := strconv.ParseInt(postIdStr, 10, 64)
	if err != nil {
		return ui.ErrorPage(ctxt, fmt.Errorf("internal error converting postID: %v", err))
	}

	hdr, err := database.AmGetPost(ctxt.Ctx(), postId)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	// Retrieve attachment info and data.
	info, err := hdr.AttachmentInfo(ctxt.Ctx())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	} else if info == nil {
		return ui.ErrorPage(ctxt, errors.New("attachment not found"))
	}
	data, err := hdr.AttachmentData(ctxt.Ctx())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	// Record a "hit" on this attachment in the background.
	ampool.Submit(func(ctx context.Context) {
		hdr.HitAttachment(ctx)
	})

	// Send the attachment data.
	ctxt.SetOutputType(info.MIMEType)
	if !strings.HasPrefix(info.MIMEType, "image/") {
		ctxt.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", info.Filename))
	}
	return "bytes", data, nil
}

/* HideTopic hides or shows rthe current topic for the current user.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func HideTopic(ctxt ui.AmContext) (string, any, error) {
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	hidden, err := topic.IsHidden(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	err = topic.SetHidden(ctxt.Ctx(), ctxt.CurrentUser(), !hidden)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
}
