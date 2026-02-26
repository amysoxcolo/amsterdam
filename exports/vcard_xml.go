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
	"errors"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
)

/*
 * This file defines data structures for XML vCards as defined by the XMPP Foundation specification XEP-0054, "vcard-temp",
 * https://xmpp.org/extensions/xep-0054.html.
 */

// VCard is the top level vCard structure.
type VCard struct {
	XMLName      xml.Name          `xml:"vcard-temp vCard"` // name must be "vCard" in the "vcard-temp" namespace
	Version      string            `xml:"VERSION"`          // vCard version number
	FullName     string            `xml:"FN"`               // full name
	Name         VCName            `xml:"N"`                // broken-up name components
	Nickname     string            `xml:"NICKNAME"`         // nickname (not used in Amsterdam)
	Photo        *VCPhoto          `xml:"PHOTO"`            // user photo (not used in Amsterdam)
	BDay         string            `xml:"BDAY"`             // birthday, ISO 8601 format
	Address      *[]VCAddress      `xml:"ADR"`              // address
	AddressLabel *[]VCAddressLabel `xml:"LABEL"`            // address label
	Email        *[]VCEmail        `xml:"EMAIL"`            // E-mail address
	Tel          *[]VCTelephone    `xml:"TEL"`              // telephone number
	JabberID     string            `xml:"JABBERID"`         // Jabber/XMPP address (user@host) (XMPP extension) (not used in Amsterdam)
	Mailer       string            `xml:"MAILER"`           // mailer user agent (not used in Amsterdam)
	TZ           string            `xml:"TZ"`               // time zone indicator, ISO 8601 formatted UTC offset
	Geolocation  *VCGeolocation    `xml:"GEO"`              // geolocation (not used in Amsterdam)
	Title        string            `xml:"TITLE"`            // job title (not used in Amsterdam)
	Role         string            `xml:"ROLE"`             // job role (not used in Amsterdam)
	Logo         *VCLogo           `xml:"LOGO"`             // organization logo (not used in Amsterdam)
	Agent        *VCAgent          `xml:"AGENT"`            // agent for the organization (not used in Amsterdam)
	Org          *VCOrganization   `xml:"ORG"`              // organization
	Categories   *VCCategory       `xml:"CATEGORIES"`       // categories
	Note         string            `xml:"NOTE"`             // text note (not used by Amsterdam)
	ProductID    string            `xml:"PRODID"`           // product ID that generated this vCard (not used by Amsterdam)
	LastUpdate   string            `xml:"REV"`              // last update to this information, ISO 8601 format
	SortString   string            `xml:"SORT-STRING"`      // sort string
	Sound        *VCSound          `xml:"SOUND"`            // pronunciation property (not used in Amsterdam)
	UID          string            `xml:"UID"`              // unique identifier (not necessarily an Amsterdam UID!) (not used in Amsterdam)
	URL          string            `xml:"URL"`              // URL
	Class        *VCClass          `xml:"CLASS"`            // privacy classification (not used in Amsterdam)
	Key          *VCKey            `xml:"KEY"`              // authentication credential or encryption key (not used in Amsterdam)
	Description  string            `xml:"DESC"`             // description string value (XMPP extension)
}

// VCPhoto is the "photo" attachment to the VCard.
type VCPhoto struct {
	XMLName       xml.Name `xml:"PHOTO"`  // must be a "PHOTO" tag
	Type          string   `xml:"TYPE"`   // data type for BINVAL
	BinaryValue   string   `xml:"BINVAL"` // binary photo value (Base64 encoded)
	ExternalValue string   `xml:"EXTVAL"` // external value of photo (URL)
}

// Name is the "structured name" property of the vCard.
type VCName struct {
	XMLName xml.Name `xml:"N"`      // must be an "N" tag
	Family  string   `xml:"FAMILY"` // family name
	Given   string   `xml:"GIVEN"`  // given name
	Middle  string   `xml:"MIDDLE"` // middle name/initial
	Prefix  string   `xml:"PREFIX"` // prefix
	Suffix  string   `xml:"SUFFIX"` // suffix
}

// VCAddress is the "address" property of the vCard.
type VCAddress struct {
	XMLName       xml.Name `xml:"ADR"`      // must be a "ADR" tag
	Work          xml.Name `xml:"WORK"`     // Presence indicates work address
	Home          xml.Name `xml:"HOME"`     // Presence indicates home address
	Postal        xml.Name `xml:"POSTAL"`   // presence indicates postal address
	Parcel        xml.Name `xml:"PARCEL"`   // presence indicates parcel address
	Domestic      xml.Name `xml:"DOM"`      // presence indicates domestic address
	International xml.Name `xml:"INTL"`     // presence indicates international address
	Preferred     xml.Name `xml:"PREF"`     // Presence indicates preferred address
	POBox         string   `xml:"POBOX"`    // post office box
	Locality      string   `xml:"LOCALITY"` // locality (city)
	Region        string   `xml:"REGION"`   // region (state/province)
	PCode         string   `xml:"PCODE"`    // postal code
	Country       string   `xml:"CTRY"`     // country
	Street        string   `xml:"STREET"`   // street address (addr line 1)
	ExtAddr       string   `xml:"EXTADR"`   // extended address (addr line 2)
}

// VCAddressLabel is the "address label" property of the vCard.
type VCAddressLabel struct {
	XMLName       xml.Name `xml:"LABEL"`  // must be a "LABEL" tag
	Work          xml.Name `xml:"WORK"`   // Presence indicates work address
	Home          xml.Name `xml:"HOME"`   // Presence indicates home address
	Postal        xml.Name `xml:"POSTAL"` // presence indicates postal address
	Parcel        xml.Name `xml:"PARCEL"` // presence indicates parcel address
	Domestic      xml.Name `xml:"DOM"`    // presence indicates domestic address
	International xml.Name `xml:"INTL"`   // presence indicates international address
	Preferred     xml.Name `xml:"PREF"`   // Presence indicates preferred address
	Lines         []string `xml:"LINE"`   // lines of text on the address label
}

// VCEmail is the "E-mail address" property of the vCard.
type VCEmail struct {
	XMLName   xml.Name `xml:"EMAIL"`    // must be an "EMAIL" tag
	Work      xml.Name `xml:"WORK"`     // presence indicates work address
	Home      xml.Name `xml:"HOME"`     // presence indicates home address
	Internet  xml.Name `xml:"INTERNET"` // presence indicates Internet address
	Preferred xml.Name `xml:"PREF"`     // Presence indicates preferred address
	X400      xml.Name `xml:"X400"`     // Presence indicates X.400 address
	UserID    string   `xml:"USERID"`   // user ID (address)
}

// VCTelephone is the "telephone number" property of the vCard.
type VCTelephone struct {
	XMLName   xml.Name `xml:"TEL"`    // must be a "TEL" tag
	Work      xml.Name `xml:"WORK"`   // presence indicates work number
	Home      xml.Name `xml:"HOME"`   // presence indicates home number
	Voice     xml.Name `xml:"VOICE"`  // presence indicates voice number
	Fax       xml.Name `xml:"FAX"`    // presence indicates fax number
	Pager     xml.Name `xml:"PAGER"`  // presence indicates pager number
	Message   xml.Name `xml:"MSG"`    // presence indicates message number
	Cell      xml.Name `xml:"CELL"`   // presence indicates cellphone number
	Video     xml.Name `xml:"VIDEO"`  // presence indicates videophone number
	BBS       xml.Name `xml:"BBS"`    // presence indicates BBS number
	Modem     xml.Name `xml:"MODEM"`  // presence indicates modem number
	ISDN      xml.Name `xml:"ISDN"`   // presence indicates ISDN number
	PCS       xml.Name `xml:"PCS"`    // presence indicates PCS number
	Preferred xml.Name `xml:"PREF"`   // presence indicates preferred number
	Number    string   `xml:"NUMBER"` // the number
}

// VCGeolocation is the "geolocation" property of the vCard.
type VCGeolocation struct {
	XMLName   xml.Name `xml:"GEO"`  // must be a "GEO" tag
	Latitude  string   `xml:"LAT"`  // latitude to six decimal places (North is positive)
	Longitude string   `xml:"LONG"` // longitude to six decimal places (East is positive)
}

// VCLogo is the "logo" property of the vCard.
type VCLogo struct {
	XMLName       xml.Name `xml:"LOGO"`   // must be a "LOGO" tag
	Type          string   `xml:"TYPE"`   // data type for BINVAL
	BinaryValue   string   `xml:"BINVAL"` // binary photo value (Base64 encoded)
	ExternalValue string   `xml:"EXTVAL"` // external value of photo (URL)
}

// VCAgent is the "agent" property of the vCard.
type VCAgent struct {
	XMLName       xml.Name `xml:"AGENT"`            // must be an "AGENT" tag
	VCard         *VCard   `xml:"vcard-temp vCard"` // vCard with agent contact info
	ExternalValue string   `xml:"EXTVAL"`           // external value, such as URL to contact info
}

// VCOrganization is the "organization" property of the vCard.
type VCOrganization struct {
	XMLName xml.Name  `xml:"ORG"`     // must be an "ORG" tag
	OrgName string    `xml:"ORGNAME"` // organization name
	OrgUnit *[]string `xml:"ORGUNIT"` // organization unit(s)
}

// VCCategory is the "category" property of the vCard.
type VCCategory struct {
	XMLName  xml.Name `xml:"CATEGORIES"` // must be a "CATEGORIES" tag
	Keywords []string `xml:"KEYWORD"`    // keywords
}

// VCSound is the "pronunciation guide" property of the vCard.
type VCSound struct {
	XMLName       xml.Name `xml:"SOUND"`    // must be a "SOUND" tag
	Phonetic      string   `xml:"PHONETIC"` // phonetic pronunciation
	BinaryValue   string   `xml:"BINVAL"`   // binary audio value (Base64 encoded)
	ExternalValue string   `xml:"EXTVAL"`   // external value of audio (URL)
}

// VCClass is the "privacy classification" property of the vCard.
type VCClass struct {
	XMLName      xml.Name `xml:"CLASS"`        // must be a "CLASS" tag
	Public       xml.Name `xml:"PUBLIC"`       // presence indicates public information
	Private      xml.Name `xml:"PRIVATE"`      // presence indicates private information
	Confidential xml.Name `xml:"CONFIDENTIAL"` // presence indicates confidential information
}

// VCKey is the "authentication or encryption key" property of the vCard.
type VCKey struct {
	XMLName    xml.Name `xml:"KEY"`  // must be a "KEY" tag
	Type       string   `xml:"TYPE"` // type indicator
	Credential string   `xml:"CRED"` // credential value
}

func VCardFromContactInfo(ctx context.Context, target *VCard, ci *database.ContactInfo) error {
	target.Version = "2.0"
	target.FullName = ci.FullName(false)
	target.Name.Family = util.SRef(ci.FamilyName)
	target.Name.Given = util.SRef(ci.GivenName)
	target.Name.Middle = util.SRef(ci.MiddleInit)
	target.Name.Prefix = util.SRef(ci.Prefix)
	target.Name.Suffix = util.SRef(ci.Suffix)
	target.URL = util.SRef(ci.URL)
	if ci.LastUpdate != nil {
		target.LastUpdate = ci.LastUpdate.Format(ISO8601)
	}

	addr := make([]VCAddress, 1)
	addr[0].Home.Local = "HOME"
	addr[0].Postal.Local = "POSTAL"
	addr[0].Preferred.Local = "PREF"
	addr[0].Street = util.SRef(ci.Addr1)
	addr[0].ExtAddr = util.SRef(ci.Addr2)
	addr[0].Locality = util.SRef(ci.Locality)
	addr[0].Region = util.SRef(ci.Region)
	addr[0].PCode = util.SRef(ci.PostalCode)
	addr[0].Country = util.SRef(ci.Country)
	target.Address = &addr

	phcount := util.IIF(ci.Phone != nil, 1, 0) + util.IIF(ci.Fax != nil, 1, 0) + util.IIF(ci.Mobile != nil, 1, 0)
	if phcount > 0 {
		phone := make([]VCTelephone, phcount)
		i := 0
		if ci.Phone != nil {
			phone[i].Home.Local = "HOME"
			phone[i].Voice.Local = "VOICE"
			phone[i].Preferred.Local = "PREF"
			phone[i].Number = *ci.Phone
			i++
		}
		if ci.Fax != nil {
			phone[i].Home.Local = "HOME"
			phone[i].Fax.Local = "FAX"
			phone[i].Preferred.Local = "PREF"
			phone[i].Number = *ci.Fax
			i++
		}
		if ci.Mobile != nil {
			phone[i].Home.Local = "HOME"
			phone[i].Cell.Local = "CELL"
			phone[i].Voice.Local = "VOICE"
			phone[i].Preferred.Local = "PREF"
			phone[i].Number = *ci.Mobile
			i++
		}
		if i == phcount {
			target.Tel = &phone
		} else {
			return errors.New("internal error in phone array")
		}
	}

	if ci.Email != nil {
		email := make([]VCEmail, 1)
		email[0].Home.Local = "HOME"
		email[0].Internet.Local = "INTERNET"
		email[0].Preferred.Local = "PREF"
		email[0].UserID = *ci.Email
		target.Email = &email
	}

	if ci.Company != nil {
		var org VCOrganization
		org.OrgName = *ci.Company
		target.Org = &org
	}
	return nil
}
