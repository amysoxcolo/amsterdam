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
	"strconv"
	"strings"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/exports"
	"git.erbosoft.com/amy/amsterdam/ui"
	"git.erbosoft.com/amy/amsterdam/util"
	log "github.com/sirupsen/logrus"
)

/* EditConferenceForm displays the dialog for editing the conference properties.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func EditConferenceForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	dlg, err := ui.AmLoadDialog("edit_conference")
	if err != nil {
		return "error", err
	}
	dlg.SetCommunity(comm)
	dlg.SetConference(conf, ctxt.GetScratch("currentAlias").(string))
	dlg.Field("name").Value = conf.Name
	dlg.Field("descr").SetVal(conf.Description)
	if comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		f, err := conf.HiddenInList(ctxt.Ctx(), comm)
		if err != nil {
			return "error", err
		}
		dlg.Field("hide").SetChecked(f)
	} else {
		dlg.Field("hide").Disabled = true
	}
	dlg.Field("read_lvl").SetLevel(conf.ReadLevel)
	dlg.Field("post_lvl").SetLevel(conf.PostLevel)
	dlg.Field("create_lvl").SetLevel(conf.CreateLevel)
	dlg.Field("hide_lvl").SetLevel(conf.HideLevel)
	dlg.Field("nuke_lvl").SetLevel(conf.NukeLevel)
	dlg.Field("change_lvl").SetLevel(conf.ChangeLevel)
	dlg.Field("delete_lvl").SetLevel(conf.DeleteLevel)
	flags, err := conf.Flags(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	dlg.Field("pic_in_post").SetChecked(flags.Get(database.ConferenceFlagPicturesInPosts))
	dlg.Field("bugattach").SetChecked(flags.Get(database.ConferenceFlagBuggyAttachments))
	return dlg.Render(ctxt)
}

/* EditConference saves the conference properties being edited.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func EditConference(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	dlg, err := ui.AmLoadDialog("edit_conference")
	if err != nil {
		return "error", err
	}
	button := dlg.WhichButton(ctxt)
	if button == "cancel" {
		return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
	} else if button != "update" {
		dlg.SetCommunity(comm)
		dlg.SetConference(conf, ctxt.GetScratch("currentAlias").(string))
		return dlg.RenderError(ctxt, "invalid button pressed")
	}

	dlg.LoadFromForm(ctxt)
	if err = dlg.Validate(); err == nil {
		if err = conf.SetInfo(ctxt.Ctx(), dlg.Field("name").Value, dlg.Field("descr").Value, dlg.Field("read_lvl").GetLevel(), dlg.Field("post_lvl").GetLevel(),
			dlg.Field("create_lvl").GetLevel(), dlg.Field("hide_lvl").GetLevel(), dlg.Field("nuke_lvl").GetLevel(), dlg.Field("change_lvl").GetLevel(),
			dlg.Field("delete_lvl").GetLevel(), ctxt.CurrentUser(), comm, ctxt.RemoteIP()); err == nil {
			if err = conf.SetHiddenInList(ctxt.Ctx(), comm, dlg.Field("hide").IsChecked()); err == nil {
				var flags *util.OptionSet
				flags, err = conf.Flags(ctxt.Ctx())
				if err == nil {
					flags.Set(database.ConferenceFlagPicturesInPosts, dlg.Field("pic_in_post").IsChecked())
					flags.Set(database.ConferenceFlagBuggyAttachments, dlg.Field("bugattach").IsChecked())
					err = conf.SaveFlags(ctxt.Ctx(), flags)
				}
			}
		}
	}
	if err != nil {
		dlg.SetCommunity(comm)
		dlg.SetConference(conf, ctxt.GetScratch("currentAlias").(string))
		return dlg.RenderError(ctxt, err.Error())
	}

	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
}

/* ConferenceAliasForm displays the form for managing conference aliases.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceAliasForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	ctxt.VarMap().Set("newAlias", "")
	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/aliases", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.SetFrameTitle(fmt.Sprintf("Manage Conference Aliases: %s", conf.Name))

	if ctxt.HasParameter("del") {
		err := conf.RemoveAlias(ctxt.Ctx(), ctxt.Parameter("del"), ctxt.CurrentUser(), comm, ctxt.RemoteIP())
		if err != nil {
			ctxt.VarMap().Set("errorMessage", err.Error())
		}
	}

	aliases, err := conf.Aliases(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("aliases", aliases)
	return "framed", "conf_aliases.jet"
}

/* ConferenceAliasAdd adds a new alias to the current conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceAliasAdd(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/aliases", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.SetFrameTitle(fmt.Sprintf("Manage Conference Aliases: %s", conf.Name))

	newAlias := ctxt.FormField("na")
	ctxt.VarMap().Set("newAlias", newAlias)

	var err error = nil
	if ctxt.FormFieldIsSet("add") {
		if database.AmIsValidAmsterdamID(newAlias) {
			err = conf.AddAlias(ctxt.Ctx(), newAlias, ctxt.CurrentUser(), comm, ctxt.RemoteIP())
		} else {
			err = fmt.Errorf("value '%s' is not a valid Amsterdam id", newAlias)
		}
	} else {
		err = errors.New("invalid button press")
	}

	if err != nil {
		ctxt.VarMap().Set("errorMessage", err.Error())
	}

	aliases, err := conf.Aliases(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("newAlias", "")
	ctxt.VarMap().Set("aliases", aliases)
	return "framed", "conf_aliases.jet"
}

// CMData is the result data passed to the conference members page.
type CMData struct {
	User  *database.User
	Level uint16
}

/* ConferenceMembers shows the conference members and allows their access levels to be adjusted.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceMembers(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	// Set the first batch of page variables.
	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/members", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("roleList", database.AmRoleList("Conference.UserLevels"))
	ctxt.SetFrameTitle(fmt.Sprintf("Membership in Conference: %s", conf.Name))

	// Get the search parameter values and adjust them.
	mode := "conf"
	field := "name"
	oper := "st"
	term := ""
	offset := 0
	if ctxt.Verb() == "POST" {
		mode = ctxt.FormField("mode")
		field = ctxt.FormField("field")
		oper = ctxt.FormField("oper")
		term = ctxt.FormField("term")
		var e1 error
		offset, e1 = ctxt.FormFieldInt("ofs")
		if e1 != nil {
			offset = 0
		}
	}
	maxPage := ctxt.Globals().MaxSearchPage

	// Adjust the offset based on the page buttons.
	if ctxt.FormFieldIsSet("prev") {
		offset = max(0, offset-int(maxPage))
	} else if ctxt.FormFieldIsSet("next") {
		offset += int(maxPage)
	}

	// Write the search parameters back to the page variables.
	ctxt.VarMap().Set("mode", mode)
	ctxt.VarMap().Set("field", field)
	ctxt.VarMap().Set("oper", oper)
	ctxt.VarMap().Set("term", term)
	ctxt.VarMap().Set("offset", offset)
	ctxt.VarMap().Set("max", maxPage)

	if ctxt.FormFieldIsSet("update") {
		// Parse out the list of valid UIDs.
		uids := util.Map(strings.Split(ctxt.FormField("validUids"), "|"), func(in string) int32 {
			rc, err := strconv.Atoi(in)
			if err != nil {
				return -1
			}
			return int32(rc)
		})
		for _, uid := range uids {
			if uid > 0 {
				// Get old and new access levels from the form.
				tmp, err := ctxt.FormFieldInt(fmt.Sprintf("old_%d", uid))
				if err == nil {
					oldLevel := uint16(tmp)
					tmp, err = ctxt.FormFieldInt(fmt.Sprintf("new_%d", uid))
					if err == nil {
						newLevel := uint16(tmp)
						if oldLevel != newLevel {
							// Update the level for this user.
							var u *database.User
							u, err = database.AmGetUser(ctxt.Ctx(), uid)
							if err == nil {
								err = conf.SetMembership(ctxt.Ctx(), u, newLevel, ctxt.CurrentUser(), comm, ctxt.RemoteIP())
							}
						}
					}
				}
				if err != nil {
					return "error", err
				}
			}
		}
		ctxt.VarMap().Set("updated", true)
	}

	// Get the member list for the conference.
	members, err := conf.Members(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	// Generate the result list.
	total := 0
	var mr []CMData
	switch mode {
	case "conf":
		total = len(members)
		if offset > 0 {
			members = members[offset:]
		}
		if len(members) > int(maxPage) {
			members = members[:maxPage]
		}
		mr = make([]CMData, len(members))
		for i := range members {
			mr[i].User, _ = database.AmGetUser(ctxt.Ctx(), members[i].Uid)
			mr[i].Level = members[i].Level
		}
	case "comm":
		ulist, t, err := database.AmSearchCommunityMembers(ctxt.Ctx(), comm, SearchUserFieldMap[field], SearchUserOperMap[oper], term, offset, int(maxPage))
		if err != nil {
			return "error", err
		}
		total = t
		mr = make([]CMData, len(ulist))
		for i := range ulist {
			mr[i].User = ulist[i]
			mr[i].Level = 0
			for j := range members {
				if members[j].Uid == ulist[i].Uid {
					mr[i].Level = members[j].Level
					break
				}
			}
		}
	}

	// Set the last few variables and return.
	ctxt.VarMap().Set("resultList", mr)
	ctxt.VarMap().Set("total", total)
	ctxt.VarMap().Set("validUids", strings.Join(util.Map(mr, func(cd CMData) string {
		return fmt.Sprintf("%d", cd.User.Uid)
	}), "|"))
	if offset > 0 {
		ctxt.VarMap().Set("showPrev", true)
	}
	if (offset + len(mr)) < total {
		ctxt.VarMap().Set("showNext", true)
	}
	return "framed", "conf_members.jet"
}

/* ConfCustomForm displays the form for editing the conference's custom HTML blocks.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConfCustomForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	topBlock, bottomBlock, err := conf.GetCustomBlocks(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/custom", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("topText", topBlock)
	ctxt.VarMap().Set("bottomText", bottomBlock)
	ctxt.SetFrameTitle(fmt.Sprintf("Customize Conference: %s", conf.Name))
	return "framed", "conf_custom.jet"
}

/* ConfCustom modifies or removes the conference's custom HTML blocks.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConfCustom(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	var err error
	if ctxt.FormFieldIsSet("cancel") {
		err = nil
	} else if ctxt.FormFieldIsSet("remove") {
		err = conf.RemoveCustomBlocks(ctxt.Ctx())
	} else if ctxt.FormFieldIsSet("update") {
		err = conf.SetCustomBlocks(ctxt.Ctx(), ctxt.FormField("tx"), ctxt.FormField("bx"))
	} else {
		return "error", EBUTTON
	}
	if err != nil {
		return "error", err
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
}

/* ConfReports displays conference activity reports.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConfReports(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Read", myLevel) {
		return "error", ENOPERM
	}

	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/activity", comm.Alias, ctxt.GetScratch("currentAlias")))

	if ctxt.HasParameter("r") {
		// generate a report
		reportMode := ctxt.Parameter("r")
		var reportTypeSel int
		switch reportMode {
		case "post":
			reportTypeSel = database.ActivityReportPosters
		case "read":
			reportTypeSel = database.ActivityReportReaders
		default:
			return "error", EINVAL
		}
		ctxt.VarMap().Set("reportMode", reportMode)
		if ctxt.HasParameter("t") {
			topicId := ctxt.QueryParamInt("t", -1)
			if topicId > 0 {
				topic, err := database.AmGetTopic(ctxt.Ctx(), int32(topicId))
				if err != nil {
					return "error", err
				}
				ctxt.VarMap().Set("topic", topic)
				report, err := topic.GetActivity(ctxt.Ctx(), reportTypeSel)
				if err != nil {
					return "error", err
				}
				ctxt.VarMap().Set("report", report)
				if reportTypeSel == database.ActivityReportPosters {
					ctxt.SetFrameTitle("Users Posting in Topic " + topic.Name)
				} else {
					ctxt.SetFrameTitle("Users Reading Topic " + topic.Name)
				}
			} else {
				return "error", "Invalid topic ID specified"
			}
		} else {
			report, err := conf.GetActivity(ctxt.Ctx(), reportTypeSel)
			if err != nil {
				return "error", err
			}
			ctxt.VarMap().Set("report", report)
			if reportTypeSel == database.ActivityReportPosters {
				ctxt.SetFrameTitle("Users Posting in Conference " + conf.Name)
			} else {
				ctxt.SetFrameTitle("Users Reading Conference " + conf.Name)
			}
		}
		return "framed", "conf_reportout.jet"
	} else {
		// generate the listing
		topicList, err := database.AmListTopics(ctxt.Ctx(), conf.ConfId, ctxt.CurrentUserId(), database.TopicViewAll, database.TopicSortNumber, true)
		if err != nil {
			return "error", err
		}
		ctxt.VarMap().Set("topics", topicList)
		ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")))
		ctxt.SetFrameTitle(fmt.Sprintf("Conference Reports: %s", conf.Name))
		return "framed", "conf_reports.jet"
	}
}

/* ConferenceEmailForm displays the dialog for E-mailing participants in a conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceEmailForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.EMailParticipants", myLevel) {
		return "error", ENOPERM
	}

	topics, err := database.AmListTopics(ctxt.Ctx(), conf.ConfId, ctxt.CurrentUserId(), database.TopicViewAll, database.TopicSortName, true)
	if err != nil {
		return "error", err
	}
	ctxt.VarMap().Set("topics", topics)
	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/email", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("porl", 0).Set("top", 0).Set("xday", false)
	ctxt.VarMap().Set("day", 7).Set("subj", "").Set("pb", "")
	ctxt.SetFrameTitle(fmt.Sprintf("Conference E-Mail: %s", conf.Name))
	return "framed", "conf_email.jet"
}

/* ConferenceEmail sends E-mail to participants in a conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceEmail(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.EMailParticipants", myLevel) {
		return "error", ENOPERM
	}

	// Handle button presses.
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
	} else if !ctxt.FormFieldIsSet("send") {
		return "error", EBUTTON
	}

	// extract user selector
	porl := ctxt.FormField("porl")
	var userSelect int
	switch porl {
	case "0":
		userSelect = database.ActiveUserPosters
	case "1":
		userSelect = database.ActiveUserReaders
	default:
		return "error", EINVAL
	}

	// extract number of days
	days := -1
	if ctxt.FormFieldIsSet("xday") {
		var err error
		days, err = ctxt.FormFieldInt("day")
		if err != nil {
			return "error", err
		} else if days <= 0 {
			return "error", "Invalid number of days specified"
		}
	}

	// extract list of recipients and other needed data
	var recipients []string
	templateName := ""
	topicName := ""
	top, err := ctxt.FormFieldInt("top")
	if err != nil {
		return "error", err
	}
	if top == 0 {
		recipients, err = conf.GetActiveUserEMailAddrs(ctxt.Ctx(), userSelect, days)
		if userSelect == database.ActiveUserPosters {
			templateName = "conf_mass_poster.jet"
		} else {
			templateName = "conf_mass_reader.jet"
		}
	} else {
		var topic *database.Topic
		if topic, err = database.AmGetTopicByNumber(ctxt.Ctx(), conf, int16(top)); err == nil {
			recipients, err = topic.GetActiveUserEMailAddrs(ctxt.Ctx(), userSelect, days)
			topicName = topic.Name
		}
		if userSelect == database.ActiveUserPosters {
			templateName = "topic_mass_poster.jet"
		} else {
			templateName = "topic_mass_reader.jet"
		}
	}
	if err != nil {
		return "error", err
	}

	// Kick off a background task to send all the E-mail messages.
	subj := ctxt.FormField("subj")
	pb := ctxt.FormField("pb")
	confName := conf.Name
	commName := comm.Name
	myUID := ctxt.CurrentUserId()
	myIP := ctxt.RemoteIP()
	log.Infof("ConferenceEmail: About to send mass E-mail to %d recipients", len(recipients))
	ampool.Submit(func(ctx context.Context) {
		start := time.Now()
		for _, addr := range recipients {
			err := ctx.Err()
			if err != nil {
				break
			}
			msg := email.AmNewEmailMessage(myUID, myIP)
			msg.AddTo(addr, "")
			msg.SetSubject(subj)
			msg.SetTemplate(templateName)
			msg.AddVariable("text", pb)
			msg.AddVariable("topicName", topicName)
			msg.AddVariable("confName", confName)
			msg.AddVariable("commName", commName)
			msg.Send()
		}
		elapsed := time.Since(start)
		log.Infof("ConferenceEmail delivery completed in %s", elapsed)
	})

	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
}

/* ConferenceExportForm displays the form for exporting data from a conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceExportForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	topics, err := database.AmListTopics(ctxt.Ctx(), conf.ConfId, ctxt.CurrentUserId(), database.TopicViewAll, database.TopicSortNumber, true)
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("topics", topics)
	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/export", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.SetFrameTitle(fmt.Sprintf("Export Messages: %s", conf.Name))
	return "framed", "conf_export.jet"
}

/* ConferenceExport exports data from a conference to a downloaded VCIF file.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceExport(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
	} else if !ctxt.FormFieldIsSet("export") {
		return "error", EBUTTON
	}

	// Get the topic numbers selected.
	topicNumStrs, err := ctxt.FormFieldValues("tselect")
	if err != nil {
		return "error", err
	}
	if len(topicNumStrs) == 0 {
		return "nocontent", nil // this is a no-op
	}

	// Convert into a list of topics.
	topics := make([]*database.Topic, len(topicNumStrs))
	for i, tn := range topicNumStrs {
		tnum, err := strconv.ParseInt(tn, 10, 16)
		if err == nil {
			topics[i], err = database.AmGetTopicByNumber(ctxt.Ctx(), conf, int16(tnum))
		}
		if err != nil {
			return "error", err
		}
	}

	// Get the value of the "bug workaround" flag. If not from the command line, then from the conference flags.
	bugWorkaround := config.CommandLine.BuggyAttachments
	if !bugWorkaround {
		flg, err := conf.Flags(ctxt.Ctx())
		if err != nil {
			return "error", err
		}
		bugWorkaround = flg.Get(database.ConferenceFlagBuggyAttachments)
	}

	// The tricky bit! We use a dedicated goroutine to generate the streamed output and send it to the inlet end of a pipe.
	filename := time.Now().Format("exported-data-20060102.xml")
	r, w := io.Pipe()
	go func() {
		start := time.Now()
		err := exports.VCIFStreamTopicFile(context.Background(), w, topics, bugWorkaround)
		if err != nil {
			log.Errorf("ConferenceExport task failed with %v", err)
			s := fmt.Sprintf("<!-- ***PROCESSING ERROR*** %v -->\r\n", err)
			w.Write([]byte(s))
		}
		w.Close()
		dur := time.Since(start)
		log.Infof("ConferenceExport task completed in %v", dur)
	}()

	// Now we connect the outlet end of the pipe to the output to the browser.
	ctxt.SetOutputType("text/xml")
	ctxt.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	return "stream", r
}

/* DeleteConference handles the deletion of a conference from its operations menu.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func DeleteConference(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Delete", myLevel) {
		return "error", ENOPERM
	}

	// Load the message box, and, if we have a valid "yes," then perform the delete
	mbox, err := ui.AmLoadMessageBox("deleteConf")
	if err != nil {
		return "error", err
	}
	if mbox.Validate(ctxt, "yes") {
		err := conf.Delete(ctxt.Ctx(), comm, ctxt.CurrentUser(), ctxt.RemoteIP(), ampool)
		if err != nil {
			return "error", err
		}
		return "redirect", fmt.Sprintf("/comm/%s/conf", ctxt.CurrentCommunity().Alias)
	}

	// Set up to display the message box.
	mbox.SetMessage(fmt.Sprintf(`You are about to delete the conference <span class="font-bold text-red-600">"%s"</span>
		from the <span class="font-bold text-red-600">"%s"</span> community!`, conf.Name, comm.Name))
	mbox.SetLink("no", fmt.Sprintf("/comm/%s/conf/%s/manage", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias")))
	mbox.SetLink("yes", fmt.Sprintf("/comm/%s/conf/%s/delete", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias")))
	return mbox.Render(ctxt)
}

/* CreateConferenceForm displays the dialog for creating a new conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func CreateConferenceForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		return "error", ENOPERM
	}

	dlg, err := ui.AmLoadDialog("create_conference")
	if err != nil {
		return "error", err
	}
	dlg.SetCommunity(comm)
	return dlg.Render(ctxt)
}

/* CreateConference creates a new conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func CreateConference(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		return "error", ENOPERM
	}

	dlg, err := ui.AmLoadDialog("create_conference")
	if err != nil {
		return "error", err
	}
	button := dlg.WhichButton(ctxt)
	if button == "cancel" {
		return "redirect", fmt.Sprintf("/comm/%s/conf", comm.Alias)
	} else if button != "create" {
		dlg.SetCommunity(comm)
		return dlg.RenderError(ctxt, "invalid button pressed")
	}
	dlg.LoadFromForm(ctxt)
	alias := dlg.Field("alias").Value
	conf, err := database.AmCreateConference(ctxt.Ctx(), comm, dlg.Field("name").Value, alias, dlg.Field("descr").Value,
		dlg.Field("ctype").Value == "1", dlg.Field("hide").IsChecked(), ctxt.CurrentUser(), ctxt.RemoteIP())
	if err != nil {
		dlg.SetCommunity(comm)
		return dlg.RenderError(ctxt, err.Error())
	}
	log.Infof("Created conference '%s'", conf.Name)
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, alias)
}

/* ManageConferenceList displays the list for managing conferences.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ManageConferenceList(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		return "error", ENOPERM
	}

	if ctxt.HasParameter("t") {
		confid := ctxt.QueryParamInt("t", -1)
		if confid == -1 {
			return "error", EINVAL
		}
		conf, err := database.AmGetConference(ctxt.Ctx(), int32(confid))
		if err != nil {
			return "error", err
		}
		f, err := conf.HiddenInList(ctxt.Ctx(), comm)
		if err == nil {
			err = conf.SetHiddenInList(ctxt.Ctx(), comm, !f)
		}
		if err != nil {
			return "error", err
		}
	}

	clist, err := database.AmListConferences(ctxt.Ctx(), comm.Id, true)
	if err != nil {
		return "error", err
	}

	if ctxt.HasParameter("m") {
		index := ctxt.QueryParamInt("m", -1)
		if index == -1 {
			return "error", EINVAL
		}
		delta := ctxt.QueryParamInt("n", 0)
		if delta == 0 {
			return "error", EINVAL
		}
		err = database.AmReorderConferences(ctxt.Ctx(), comm.Id, clist[index].Sequence, clist[index+delta].Sequence)
		if err != nil {
			return "error", err
		}
		tmp := clist[index]
		clist[index] = clist[index+delta]
		clist[index+delta] = tmp
	}

	ntopics := make([]int, len(clist))
	nposts := make([]int, len(clist))
	for i, c := range clist {
		conf, err := c.Conf(ctxt.Ctx())
		if err != nil {
			return "error", err
		}
		ntopics[i], nposts[i], err = conf.Stats(ctxt.Ctx())
		if err != nil {
			return "error", err
		}
	}
	ctxt.VarMap().Set("confs", clist)
	ctxt.VarMap().Set("ntopics", ntopics)
	ctxt.VarMap().Set("nposts", nposts)
	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("baseUrl", fmt.Sprintf("/comm/%s/manage_conf", comm.Alias))
	ctxt.VarMap().Set("returnUrl", fmt.Sprintf("/comm/%s/conf", comm.Alias))
	ctxt.SetFrameTitle("Manage Conference List")
	return "framed", "manage_conflist.jet"
}

/* ManageDeleteConference handles the deletion of a conference from the management menu.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ManageDeleteConference(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Delete", myLevel) {
		return "error", ENOPERM
	}

	// Load the message box, and, if we have a valid "yes," then perform the delete
	mbox, err := ui.AmLoadMessageBox("deleteConf")
	if err != nil {
		return "error", err
	}
	if mbox.Validate(ctxt, "yes") {
		err := conf.Delete(ctxt.Ctx(), comm, ctxt.CurrentUser(), ctxt.RemoteIP(), ampool)
		if err != nil {
			return "error", err
		}
		return "redirect", fmt.Sprintf("/comm/%s/manage_conf", ctxt.CurrentCommunity().Alias)
	}

	// Set up to display the message box.
	mbox.SetMessage(fmt.Sprintf(`You are about to delete the conference <span class="font-bold text-red-600">"%s"</span>
		from the <span class="font-bold text-red-600">"%s"</span> community!`, conf.Name, comm.Name))
	mbox.SetLink("no", fmt.Sprintf("/comm/%s/manage_conf", ctxt.CurrentCommunity().Alias))
	mbox.SetLink("yes", fmt.Sprintf("/comm/%s/manage_conf/del/%s", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias")))
	return mbox.Render(ctxt)
}
