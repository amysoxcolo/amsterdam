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

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
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
	XMLName          xml.Name           `xml:"venice-user"`       // must be a <venice-user> tag
	ID               int                `xml:"id,attr"`           // the UID for the user
	Username         string             `xml:"username"`          // user name
	Password         VIUPassword        `xml:"password"`          // password information
	PasswordReminder string             `xml:"password-reminder"` // password reminder string
	Description      string             `xml:"description"`       // description string
	Options          VIUOptions         `xml:"options"`           // user options
	VCard            VCard              `xml:"vcard-temp vCard"`  // user contact info in vCard XML format
	Joins            []VIUCommunityJoin `xml:"join"`              // joined communities
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

func VIUUserFromUser(ctx context.Context, target *VIUUser, u *database.User) error {
	// Fill base fields first.
	target.ID = int(u.Uid)
	target.Username = u.Username
	target.Password.Prehashed = true
	target.Password.Hash = u.Passhash
	target.PasswordReminder = u.PassReminder
	target.Description = util.IIF(u.Description != nil, *u.Description, "")

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

	target.Options.PostPictures = flags.Get(database.UserFlagPicturesInPosts)
	target.Options.OptOut = flags.Get(database.UserFlagMassMailOptOut)
	target.Options.NoPhoto = flags.Get(database.UserFlagDisallowSetPhoto)

	// TODO - fill in Joins
	return nil
}
