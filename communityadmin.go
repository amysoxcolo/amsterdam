/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
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
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/exports"
	"git.erbosoft.com/amy/amsterdam/ui"
	"git.erbosoft.com/amy/amsterdam/util"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

/* ExportCommunityMembers exports the members of the community as XML.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ExportCommunityMembers(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.ShowAdmin", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}

	// use a dedicated goroutine to generate the streamed XML and send it into one end of a pipe
	filename := time.Now().Format("exported-users-20060102.xml")
	r, w := io.Pipe()
	go func() {
		start := time.Now()
		err := exports.VIUStreamCommunityMemberList(context.Background(), w, comm)
		if err != nil {
			log.Errorf("ExportCommunityMembers task failed with %v", err)
			s := fmt.Sprintf("<!-- ***PROCESSING ERROR*** %v -->\r\n", err)
			w.Write([]byte(s))
		}
		w.Close()
		dur := time.Since(start)
		log.Infof("ExportCommunityMembers task completed in %v", dur)
	}()

	// Now we connect the outlet end of the pipe to the output to the browser.
	ctxt.SetOutputType("text/xml")
	ctxt.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	return "stream", r
}

/* CommunityAdminMenu renders the community administration menu.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func CommunityAdminMenu(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.ShowAdmin", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}
	menu := ui.AmMenu("communityadmin")
	defs := make(map[string]bool)
	if !ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories) {
		defs["USECAT"] = true
	}
	ctxt.VarMap().Set("menu", menu.FilterCommunity(comm))
	ctxt.VarMap().Set("defs", defs)
	ctxt.SetFrameTitle(menu.Title + " - " + comm.Name)
	return "framed", "menu.jet"
}

// setupCommunityProfileDialog sets up fields in the Community Profile dialog.
func setupCommunityProfileDialog(dlg *ui.Dialog, comm *database.Community) {
	dlg.SetCommunity(comm)
	if comm.IsAdmin {
		dlg.Field("comtype").Disabled = true
		dlg.Field("joinkey").Disabled = true
		dlg.Field("membersonly").Disabled = true
		dlg.Field("hidemode").Disabled = true
		dlg.Field("read_lvl").Disabled = true
		dlg.Field("write_lvl").Disabled = true
		dlg.Field("create_lvl").Disabled = true
		dlg.Field("delete_lvl").Disabled = true
		dlg.Field("join_lvl").Disabled = true
	}
}

// communityLogoURL returns the logo URL from the contact info, or a default.
func communityLogoURL(ci *database.ContactInfo) string {
	if ci.PhotoURL != nil && *ci.PhotoURL != "" {
		return *ci.PhotoURL
	}
	return "/img/builtin/default-community.jpg"
}

/* CommunityProfileForm displays the dfialog for editing the community profile.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func CommunityProfileForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity() // set by middleware
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}
	var ci *database.ContactInfo
	ci, err := comm.ContactInfo(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	flags, err := comm.Flags(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	dlg, err := ui.AmLoadDialog("commprofile")
	if err == nil {
		setupCommunityProfileDialog(dlg, comm)
		dlg.Field("name").Value = comm.Name
		dlg.Field("alias").Value = comm.Alias
		dlg.Field("synopsis").SetVal(comm.Synopsis)
		dlg.Field("rules").SetVal(comm.Rules)
		dlg.Field("language").SetVal(comm.Language)
		dlg.Field("url").SetVal(ci.URL)
		dlg.Field("logo").Value = communityLogoURL(ci)
		dlg.Field("company").SetVal(ci.Company)
		dlg.Field("addr1").SetVal(ci.Addr1)
		dlg.Field("addr2").SetVal(ci.Addr2)
		dlg.Field("loc").SetVal(ci.Locality)
		dlg.Field("reg").SetVal(ci.Region)
		dlg.Field("pcode").SetVal(ci.PostalCode)
		dlg.Field("country").SetVal(ci.Country)
		if comm.Public() {
			dlg.Field("comtype").Value = "0"
			dlg.Field("joinkey").Value = ""
		} else {
			dlg.Field("comtype").Value = "1"
			dlg.Field("joinkey").SetVal(comm.JoinKey)
		}
		dlg.Field("membersonly").SetChecked(comm.MembersOnly)
		if comm.HideFromSearch {
			dlg.Field("hidemode").Value = "BOTH"
		} else if comm.HideFromDirectory {
			dlg.Field("hidemode").Value = "DIRECTORY"
		} else {
			dlg.Field("hidemode").Value = "NONE"
		}
		dlg.Field("read_lvl").Value = fmt.Sprintf("%d", comm.ReadLevel)
		dlg.Field("write_lvl").Value = fmt.Sprintf("%d", comm.WriteLevel)
		dlg.Field("create_lvl").Value = fmt.Sprintf("%d", comm.CreateLevel)
		dlg.Field("delete_lvl").Value = fmt.Sprintf("%d", comm.DeleteLevel)
		dlg.Field("join_lvl").Value = fmt.Sprintf("%d", comm.JoinLevel)
		dlg.Field("pic_in_post").SetChecked(flags.Get(database.CommunityFlagPicturesInPosts))
		return dlg.Render(ctxt)
	}
	return "error", err
}

// validateJoinKey is an extra validation step for the join key.
func validateJoinKey(dlg *ui.Dialog) error {
	if dlg.Field("comtype").Value == "1" {
		if dlg.Field("joinkey").IsEmpty() {
			return errors.New("private community must specify a join key")
		}
	} else {
		dlg.Field("joinkey").Value = ""
	}
	return nil
}

/* EditCommunityProfile updates the community's profile from the dialog.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func EditCommunityProfile(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity() // set by middleware
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}
	dlg, err := ui.AmLoadDialog("commprofile")
	if err == nil {
		setupCommunityProfileDialog(dlg, comm)
		dlg.LoadFromForm(ctxt)

		action := dlg.WhichButton(ctxt)
		if action == "cancel" {
			return "redirect", fmt.Sprintf("/comm/%s/admin", comm.Alias)
		}
		if action == "update" {
			err = dlg.Validate()
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			err = validateJoinKey(dlg)
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			var ci *database.ContactInfo
			ci, err = comm.ContactInfo(ctxt.Ctx())
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			var flags *util.OptionSet
			flags, err = comm.Flags(ctxt.Ctx())
			if err != nil {
				return "error", err
			}
			nci := ci.Clone()
			nci.URL = dlg.Field("url").ValPtr()
			nci.Company = dlg.Field("company").ValPtr()
			nci.Addr1 = dlg.Field("addr1").ValPtr()
			nci.Addr2 = dlg.Field("addr2").ValPtr()
			nci.Locality = dlg.Field("loc").ValPtr()
			nci.Region = dlg.Field("reg").ValPtr()
			nci.PostalCode = dlg.Field("pcode").ValPtr()
			nci.Country = dlg.Field("country").ValPtr()
			_, err = nci.Save(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP())
			ci = nci
			if err == nil {
				var joinkey *string = nil
				if dlg.Field("comtype").Value == "1" {
					joinkey = dlg.Field("joinkey").ValPtr()
				}
				var hidedir, hidesearch bool
				switch dlg.Field("hidemode").Value {
				case "NONE":
					hidedir = false
					hidesearch = false
				case "DIRECTORY":
					hidedir = false
					hidesearch = true
				case "BOTH":
					hidedir = true
					hidesearch = true
				}
				err = comm.SetProfileData(ctxt.Ctx(), dlg.Field("name").Value, dlg.Field("alias").Value, dlg.Field("synopsis").ValPtr(),
					dlg.Field("rules").ValPtr(), dlg.Field("language").ValPtr(), joinkey, dlg.Field("membersonly").IsChecked(),
					hidedir, hidesearch, dlg.Field("read_lvl").GetLevel(), dlg.Field("write_lvl").GetLevel(),
					dlg.Field("create_lvl").GetLevel(), dlg.Field("delete_lvl").GetLevel(), dlg.Field("join_lvl").GetLevel(),
					ctxt.CurrentUser(), ctxt.RemoteIP())
			}
			if err == nil {
				flags.Set(database.CommunityFlagPicturesInPosts, dlg.Field("pic_in_post").IsChecked())
				err = comm.SaveFlags(ctxt.Ctx(), flags)
			}
			if err == nil {
				err = comm.TouchUpdate(ctxt.Ctx())
			}
			if err != nil {
				ctxt.ClearCommunityContext()
				return dlg.RenderError(ctxt, err.Error())
			} else {
				return "redirect", fmt.Sprintf("/comm/%s/admin", comm.Alias)
			}
		}
		return dlg.RenderError(ctxt, "No known button click on POST to community profile.")
	}
	return "error", err
}

/* CommunityLogoForm renders the form for changing the community logo.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func CommunityLogoForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity() // set by middleware
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}
	ci, err := comm.ContactInfo(ctxt.Ctx())
	if err == nil {
		ctxt.VarMap().Set("commName", comm.Name)
		ctxt.VarMap().Set("commAlias", comm.Alias)
		ctxt.VarMap().Set("logo_url", communityLogoURL(ci))
		ctxt.SetFrameTitle("Upload Community Logo: " + comm.Name)
		return "framed", "logo_upload.jet"
	}
	return "error", err
}

/* EditCommunityLogo handles setting the community logo.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func EditCommunityLogo(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity() // set by middleware
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}
	ci, err := comm.ContactInfo(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", "/comm/" + comm.Alias + "/admin/profile"
	}
	if ctxt.FormFieldIsSet("upload") {
		file, err := ctxt.FormFile("thepic")
		if err == nil {
			var imageData []byte
			var mimeType string
			imageData, mimeType, err = ui.AmProcessUploadedImage(file, ui.CommunityLogoWidth, ui.CommunityLogoHeight,
				ui.CommunityLogoMaxBytes)
			if err == nil {
				var img *database.ImageStore
				img, err = database.AmStoreImage(ctxt.Ctx(), database.ImageTypeCommunityLogo, comm.Id, mimeType, imageData)
				if err == nil {
					photourl := fmt.Sprintf("/img/store/%d", img.ImgId)
					ci.PhotoURL = &photourl
					_, err = ci.Save(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP())
					if err == nil {
						err = comm.TouchUpdate(ctxt.Ctx())
					}
					if err == nil {
						return "redirect", "/comm/" + comm.Alias + "/admin/profile"
					}
				}
			}
		}
		ctxt.VarMap().Set("errorMessage", err.Error())
		ctxt.VarMap().Set("commName", comm.Name)
		ctxt.VarMap().Set("commAlias", comm.Alias)
		ctxt.VarMap().Set("logo_url", communityLogoURL(ci))
		ctxt.SetFrameTitle("Upload Community Logo: " + comm.Name)
		return "framed", "logo_upload.jet"
	}
	if ctxt.FormFieldIsSet("remove") {
		purl := ci.PhotoURL
		happy := false
		if purl == nil || *purl == "" {
			// this is a no-op
			return "redirect", "/comm/" + comm.Alias + "/admin/profile"
		}
		if strings.HasPrefix(*purl, "/img/store/") {
			id, err := strconv.Atoi((*purl)[11:])
			if err != nil {
				return "error", err
			}
			defer func() {
				if happy {
					ampool.Submit(func(context.Context) {
						err := database.AmDeleteImage(ctxt.Ctx(), int32(id))
						if err != nil {
							log.Errorf("unable to delete image ID %d: %v", id, err)
						}
					})
				}
			}()
		}
		ci.PhotoURL = nil
		_, err := ci.Save(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP())
		if err != nil {
			return "error", err
		}
		happy = true
		return "redirect", "/comm/" + comm.Alias + "/admin/profile"
	}
	return "error", EBUTTON
}

/* CommunityAudit handles displaying the community audit records.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func CommunityAudit(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Delete", ctxt.EffectiveLevel()) && !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) && !comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}
	ofs := 0
	maxRecs := ctxt.Globals().NumAuditPage
	if ctxt.Verb() == "POST" {
		if ctxt.FormFieldIsSet("prev") {
			ofs = min(0, ofs-int(maxRecs))
		} else if ctxt.FormFieldIsSet("next") {
			ofs += int(maxRecs)
		}
	}

	auditRecs, total, err := database.AmListAuditRecords(ctxt.Ctx(), comm, ofs, int(maxRecs))
	if err != nil {
		return "error", err
	}

	descr := make([]string, len(auditRecs))
	userName := make([]string, len(auditRecs))
	communityName := make([]string, len(auditRecs))
	for i, ar := range auditRecs {
		descr[i] = database.AmAuditText(int(ar.Event))
		if ar.Uid > 0 {
			user, err := database.AmGetUser(ctxt.Ctx(), ar.Uid)
			if err != nil {
				userName[i] = fmt.Sprintf("<<%v>>", err)
			} else {
				userName[i] = user.Username
			}
		} else {
			userName[i] = ""
		}
		if ar.CommId > 0 {
			comm, err := database.AmGetCommunity(ctxt.Ctx(), ar.CommId)
			if err != nil {
				communityName[i] = fmt.Sprintf("<<%v>>", err)
			} else {
				communityName[i] = comm.Name
			}
		} else {
			communityName[i] = ""
		}
	}

	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/admin", comm.Alias))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/admin/audit", comm.Alias))
	ctxt.VarMap().Set("total", total)
	ctxt.VarMap().Set("ofs", ofs)
	ctxt.VarMap().Set("auditRecords", auditRecs)
	ctxt.VarMap().Set("descr", descr)
	ctxt.VarMap().Set("user", userName)
	ctxt.VarMap().Set("community", communityName)
	if ofs > 0 {
		ctxt.VarMap().Set("showPrev", true)
	}
	if ofs+int(maxRecs) < total {
		ctxt.VarMap().Set("showNext", true)
	}
	ctxt.SetFrameTitle(fmt.Sprintf("Audit Records for Community \"%s\"", comm.Name))
	return "framed", "audit.jet"
}

/* CommunityCategory handles setting the community category.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func CommunityCategory(ctxt ui.AmContext) (string, any) {
	if ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories) {
		return "error", "This instance of Amsterdam does not use the categorization system."
	}
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}

	if setId := ctxt.QueryParamInt("set", -1); setId >= 0 {
		err := comm.SetCategory(ctxt.Ctx(), int32(setId), ctxt.CurrentUser(), ctxt.RemoteIP())
		if err != nil {
			return "error", err
		}
		return "redirect", fmt.Sprintf("/comm/%s/admin", comm.Alias)
	}

	currentCat, err := database.AmGetCategory(ctxt.Ctx(), comm.CategoryId)
	if err != nil {
		return "error", err
	}

	displayId := ctxt.QueryParamInt("d", int(comm.CategoryId))
	var newCat []*database.Category
	if displayId >= 0 {
		newCat, err = database.AmGetCategoryHierarchy(ctxt.Ctx(), int32(displayId))
		if err != nil {
			return "error", err
		}
	} else {
		newCat = make([]*database.Category, 0)
	}

	subCats, err := database.AmGetSubCategories(ctxt.Ctx(), int32(displayId))
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("oldCat", currentCat)
	ctxt.VarMap().Set("newCat", newCat)
	ctxt.VarMap().Set("newCatId", displayId)
	ctxt.VarMap().Set("subCats", subCats)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/admin", comm.Alias))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/admin/category", comm.Alias))
	ctxt.SetFrameTitle("Set Community Category")
	return "framed", "comm_category.jet"
}

func CommunityMembers(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}

	// Set the first batch of page variables.
	hostRole := database.AmRole("Community.Host")
	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/admin", comm.Alias))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/admin/members", comm.Alias))
	ctxt.VarMap().Set("roleList", database.AmRoleList("Community.Userlevels"))
	ctxt.VarMap().Set("hostRole", hostRole)
	ctxt.SetFrameTitle(fmt.Sprintf("Membership in Community: %s", comm.Name))

	// Get the search parameter values and adjust them.
	mode := "comm"
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
					if oldLevel == hostRole.Level() {
						tmp = int(oldLevel)
					} else {
						tmp, err = ctxt.FormFieldInt(fmt.Sprintf("new_%d", uid))
					}
					if err == nil {
						newLevel := uint16(tmp)
						oldLock := ctxt.FormField(fmt.Sprintf("oldlock_%d", uid)) == "1"
						newLock := ctxt.FormField(fmt.Sprintf("lock_%d", uid)) == "Y"
						if (oldLevel != newLevel) || (oldLock != newLock) {
							// Update the level for this user.
							var u *database.User
							u, err = database.AmGetUser(ctxt.Ctx(), uid)
							if err == nil {
								err = comm.SetMembership(ctxt.Ctx(), u, newLevel, newLock, ctxt.CurrentUserId(), ctxt.RemoteIP())
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

	// Generate the result list.
	total := 0
	var err error
	var userlist []*database.User
	switch mode {
	case "comm":
		userlist, total, err = comm.ListMembers(ctxt.Ctx(), database.ListMembersFieldNone, database.ListMembersOperNone, "", offset, int(maxPage), false)
	case "user":
		userlist, total, err = database.AmSearchUsers(ctxt.Ctx(), SearchUserFieldMap[field], SearchUserOperMap[oper], term, offset, int(maxPage))
	}
	if err != nil {
		return "error", err
	}
	mr := make([]CMData, len(userlist))
	for i := range userlist {
		mr[i].User = userlist[i]
		var mbr bool
		mbr, mr[i].Lock, mr[i].Level, err = comm.Membership(ctxt.Ctx(), userlist[i])
		if err != nil {
			return "error", err
		}
		if !mbr {
			mr[i].Level = 0
			mr[i].Lock = false
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
	return "framed", "comm_members.jet"
}

/* CommunityEmailForm displays the form for sending mass mail to the community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func CommunityEmailForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.MassMail", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}

	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/admin/massmail", comm.Alias))
	ctxt.VarMap().Set("subj", "")
	ctxt.VarMap().Set("pb", "")
	ctxt.SetFrameTitle("Community E-Mail: " + comm.Name)
	return "framed", "comm_email.jet"
}

/* CommunityEmail sends mass mail to the community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func CommunityEmail(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.MassMail", ctxt.EffectiveLevel()) {
		return "error", ENOACCESS
	}

	// Handle button presses.
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", fmt.Sprintf("/comm/%s/admin", comm.Alias)
	} else if !ctxt.FormFieldIsSet("send") {
		return "error", EBUTTON
	}

	recipients, err := comm.GetMemberEMailAddrs(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	// Kick off a background task to send all the E-mail messages.
	subj := ctxt.FormField("subj")
	pb := ctxt.FormField("pb")
	commName := comm.Name
	myUID := ctxt.CurrentUserId()
	myIP := ctxt.RemoteIP()
	log.Infof("CommunityEmail: About to send mass E-mail to %d recipients", len(recipients))
	ampool.Submit(func(ctx context.Context) {
		start := time.Now()
	RunLoop:
		for _, addr := range recipients {
			select {
			case <-ctx.Done():
				break RunLoop
			default:
				msg := email.AmNewEmailMessage(myUID, myIP)
				msg.AddTo(addr, "")
				msg.SetSubject(subj)
				msg.SetTemplate("comm_massmail.jet")
				msg.AddVariable("text", pb)
				msg.AddVariable("commName", commName)
				msg.Send()
			}
		}
		elapsed := time.Since(start)
		log.Infof("CommunityEmail delivery completed in %s", elapsed)
	})

	return "redirect", fmt.Sprintf("/comm/%s/admin", comm.Alias)
}

/* CreateCommunityForm renders the form for creating a new community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func CreateCommunityForm(ctxt ui.AmContext) (string, any) {
	user := ctxt.CurrentUser()
	if user.BaseLevel < uint16(ctxt.Globals().CommunityCreateLevel) {
		return "error", echo.NewHTTPError(http.StatusForbidden, "you are not permitted to create a community")
	}
	dlg, err := ui.AmLoadDialog("create_comm")
	if err == nil {
		dlg.Field("language").Value = "en-US"
		dlg.Field("country").Value = "US"
		return dlg.Render(ctxt)
	}
	return "error", err
}

/* CreateCommunity creates a new community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func CreateCommunity(ctxt ui.AmContext) (string, any) {
	user := ctxt.CurrentUser()
	if user.BaseLevel < uint16(ctxt.Globals().CommunityCreateLevel) {
		return "error", echo.NewHTTPError(http.StatusForbidden, "you are not permitted to create a community")
	}
	dlg, err := ui.AmLoadDialog("create_comm")
	if err == nil {
		dlg.LoadFromForm(ctxt)
		action := dlg.WhichButton(ctxt)
		if action == "cancel" {
			return "redirect", "/"
		}
		if action == "create" {
			err = dlg.Validate()
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			err = validateJoinKey(dlg)
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			var testcomm *database.Community
			testcomm, err = database.AmGetCommunityByAlias(ctxt.Ctx(), dlg.Field("alias").Value)
			if err == nil {
				return dlg.RenderError(ctxt, fmt.Sprintf("A community with the alias \"%s\" already exists; please try again.", testcomm.Alias))
			} else if err != database.ErrNoCommunity {
				return dlg.RenderError(ctxt, err.Error())
			}
			var hideDir, hideSearch bool
			switch dlg.Field("hidemode").Value {
			case "NONE":
				hideDir = false
				hideSearch = false
			case "DIRECTORY":
				hideDir = true
				hideSearch = false
			case "BOTH":
				hideDir = true
				hideSearch = true
			}
			var comm *database.Community
			comm, err = database.AmCreateCommunity(ctxt.Ctx(), dlg.Field("name").Value, dlg.Field("alias").Value, user.Uid,
				dlg.Field("language").ValPtr(), dlg.Field("synopsis").ValPtr(), dlg.Field("rules").ValPtr(),
				dlg.Field("joinkey").ValPtr(), hideDir, hideSearch, ctxt.RemoteIP())
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			ci := database.AmNewCommunityContactInfo(user.Uid, comm.Id)
			ci.Locality = dlg.Field("loc").ValPtr()
			ci.Region = dlg.Field("reg").ValPtr()
			ci.PostalCode = dlg.Field("pcode").ValPtr()
			ci.Country = dlg.Field("country").ValPtr()
			_, err = ci.Save(ctxt.Ctx(), user, ctxt.RemoteIP())
			if err == nil {
				err = comm.SetContactID(ctxt.Ctx(), ci.ContactId)
			}
			if err == nil {
				err = comm.TouchUpdate(ctxt.Ctx())
			}
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			// new community is now created! redirect to the new profile
			return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
		}
		return dlg.RenderError(ctxt, "No known button click on POST to community creation.")
	}
	return "error", err
}
