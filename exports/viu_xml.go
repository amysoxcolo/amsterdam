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
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
	log "github.com/sirupsen/logrus"
)

/*
 * This file contains structures for working with Venice-Import-Users (VIU) XML files.
 * Amsterdam uses this name for them for backward compatibility.
 */

// VIUBase is the top level structure of the Venice-Import-Users format.
type VIUBase struct {
	XMLName xml.Name  `xml:"venice-import-users"` // must be a <venice-import-users> tag
	Users   []VIUUser `xml:"venice-user"`         // the list of users
}

// VIUUser is the structure representing a single user.
type VIUUser struct {
	XMLName          xml.Name           `xml:"venice-user"`                 // must be a <venice-user> tag
	ID               int                `xml:"id,attr"`                     // the UID for the user
	Username         string             `xml:"username"`                    // user name
	Password         VIUPassword        `xml:"password,omitempty"`          // password information
	PasswordReminder string             `xml:"password-reminder,omitempty"` // password reminder string
	Description      string             `xml:"description,omitempty"`       // description string
	Options          VIUOptions         `xml:"options"`                     // user options
	VCard            VCard              `xml:"vcard-temp vCard"`            // user contact info in vCard XML format
	Joins            []VIUCommunityJoin `xml:"join,omitempty"`              // joined communities
}

// VIUPassword represents the user password information.
type VIUPassword struct {
	XMLName   xml.Name `xml:"password"`       // must be a <password> tag
	Prehashed bool     `xml:"prehashed,attr"` // has this password been prehashed?
	Hash      string   `xml:",chardata"`      // password information
}

// VIUOptions represents the user options.
type VIUOptions struct {
	XMLName      xml.Name `xml:"options"`           // must be an <options> tag
	Confirmed    bool     `xml:"confirmed,attr"`    // E-mail address confirmed?
	Locked       bool     `xml:"locked,attr"`       // user account locked?
	Role         string   `xml:"role,attr"`         // user's base role
	HideAddr     bool     `xml:"hideaddr,attr"`     // hide address?
	HidePhone    bool     `xml:"hidephone,attr"`    // hide phone number?
	HideFax      bool     `xml:"hidefax,attr"`      // hide fax number?
	HideEmail    bool     `xml:"hideemail,attr"`    // hide E-mail address?
	AutoJoin     bool     `xml:"autojoin,attr"`     // auto-join communities?
	PostPictures bool     `xml:"postpictures,attr"` // show pictures in posts?
	OptOut       bool     `xml:"optout,attr"`       // opt out of mass E-mail?
	NoPhoto      bool     `xml:"nophoto,attr"`      // disallow photo uploads?
	Locale       string   `xml:"locale,attr"`       // user locale
	ZoneHint     string   `xml:"zonehint,attr"`     // user timezone hint
}

// VIUCommunityJoin represnts all the communities the user has joined.
type VIUCommunityJoin struct {
	XMLName   xml.Name `xml:"join"`      // must be a <join> tag
	Role      string   `xml:"role,attr"` // role we have in the community
	Community string   `xml:",chardata"` // name of community joined
}

// VIUUserFromUser fills in a VIUUser structure with details from the given user.
func VIUUserFromUser(ctx context.Context, target *VIUUser, u *database.User) error {
	// Fill base fields first.
	target.ID = int(u.Uid)
	target.Username = u.Username
	target.Password.Prehashed = true
	target.Password.Hash = u.Passhash
	target.PasswordReminder = u.PassReminder
	target.Description = util.SRef(u.Description)

	// Get the contact info.
	ci, err := u.ContactInfo(ctx)
	if err != nil {
		return err
	}

	// Fill the contact info into the VCard.
	err = VCardFromContactInfo(ctx, &(target.VCard), ci)
	if err != nil {
		return err
	}

	// Fill extra fields into the VCard.
	if u.DOB != nil {
		target.VCard.BDay = u.DOB.Format(ISO8601_DATE)
	}

	// Fill in the Options structure from what we have.
	target.Options.Confirmed = u.VerifyEMail
	target.Options.Locked = u.Lockout
	target.Options.Role = database.AmRoleList("Global.AllUserLevels").FindForLevel(u.BaseLevel).ID()
	target.Options.HideAddr = ci.PrivateAddr
	target.Options.HidePhone = ci.PrivatePhone
	target.Options.HideFax = ci.PrivateFax
	target.Options.HideEmail = ci.PrivateEmail
	target.Options.AutoJoin = u.VerifyEMail

	// Load user preferences.
	prefs, err := u.Prefs(ctx)
	if err != nil {
		return err
	}

	// Fill in from user preferences.
	target.Options.Locale = prefs.LocaleID
	target.Options.ZoneHint = prefs.TimeZoneID
	target.VCard.TZ = prefs.LocationISO8601Offset()

	// Load user flags.
	flags, err := u.Flags(ctx)
	if err != nil {
		return err
	}

	// Fill options from user flags.
	target.Options.PostPictures = flags.Get(database.UserFlagPicturesInPosts)
	target.Options.OptOut = flags.Get(database.UserFlagMassMailOptOut)
	target.Options.NoPhoto = flags.Get(database.UserFlagDisallowSetPhoto)

	// Get the list of communities for the user and set up the Joins list.
	comms, err := database.AmGetCommunitiesForUser(ctx, u.Uid)
	if err != nil {
		return err
	}
	target.Joins = make([]VIUCommunityJoin, len(comms))
	roles := database.AmRoleList("Community.AllUserlevels")
	for i, c := range comms {
		_, _, level, err := c.Membership(ctx, u)
		if err != nil {
			return err
		}
		target.Joins[i].Role = roles.FindForLevel(level).ID()
		target.Joins[i].Community = c.Alias
	}

	return nil
}

// VIUStreamCommunityMembersList streams a list of users of the given community to the given stream in VIU format.
func VIUStreamCommunityMemberList(ctx context.Context, w io.Writer, comm *database.Community) error {
	// Get the list of community members.
	total, err := comm.MemberCount(ctx, false)
	if err != nil {
		return err
	}
	users, _, err := comm.ListMembers(ctx, database.ListMembersFieldNone, database.ListMembersOperNone, "", 0, total, false)
	if err != nil {
		return err
	}

	// Write the header of the file.
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString("\r\n<venice-import-users>\r\n")
	_, err = w.Write([]byte(b.String()))
	if err != nil {
		return err
	}

	// Create the XML encoder.
	enc := xml.NewEncoder(w)
	enc.Indent("  ", "  ")

	// Build each user and then encode them to the output.
	for _, u := range users {
		var encodedUser VIUUser
		err = VIUUserFromUser(ctx, &encodedUser, u)
		if err != nil {
			return fmt.Errorf("error converting user %d: %v", u.Uid, err)
		}
		err = enc.Encode(encodedUser)
		if err != nil {
			log.Warnf("error dumping XML for user %d: %v", u.Uid, err)
		}
	}

	// Write the trailing tag.
	_, err = w.Write([]byte("</venice-import-users>\r\n"))
	return err
}

func VIUCreateUser(ctx context.Context, udata *VIUUser, loader *database.User, ipaddr string) error {
	if !database.AmIsValidAmsterdamID(udata.Username) {
		return fmt.Errorf("the username \"%s\" is not a valid Amsterdam ID", udata.Username)
	}
	email, err := VCardGetEmailAddress(&(udata.VCard))
	if err != nil {
		return err
	}
	ban, err := database.AmIsEmailAddressBanned(ctx, email)
	if err != nil {
		return err
	} else if ban {
		return fmt.Errorf("the E-mail address %s has been banned", email)
	}
	dob, err := VCardGetBirthday(&(udata.VCard))
	if err != nil {
		return err
	}
	pwd := udata.Password.Hash
	if udata.Password.Prehashed {
		pwd = ""
	}

	user, err := database.AmCreateNewUser(ctx, udata.Username, pwd, udata.PasswordReminder, dob, ipaddr)
	if err != nil {
		return err
	}
	ci := database.AmNewUserContactInfo(user.Uid)
	VCardSetContactInfo(ci, &(udata.VCard))
	ci.PrivateAddr = udata.Options.HideAddr
	ci.PrivatePhone = udata.Options.HidePhone
	ci.PrivateFax = udata.Options.HideFax
	ci.PrivateEmail = udata.Options.HideEmail
	_, err = ci.Save(ctx, loader, ipaddr)
	if err != nil {
		return err
	}
	err = user.SetContactID(ctx, ci.ContactId)
	if err != nil {
		return err
	}
	// TODO
	return nil
}

func VIUImportUserList(ctx context.Context, r io.Reader, loader *database.User, ipaddr string) (int, []string, error) {
	dec := xml.NewDecoder(r)
	var importData VIUBase
	err := dec.Decode(&importData)
	if err != nil {
		return 0, make([]string, 0), err
	}

	scroll := make([]string, 0, len(importData.Users))
	userCount := 0
	for _, udata := range importData.Users {
		err = VIUCreateUser(ctx, &udata, loader, ipaddr)
		if err != nil {
			scroll = append(scroll, fmt.Sprintf("Error creating user \"%s\": %v", udata.Username, err))
		} else {
			scroll = append(scroll, fmt.Sprintf("User \"%v\" created", udata.Username))
			userCount++
		}
	}
	return userCount, scroll, nil
}
