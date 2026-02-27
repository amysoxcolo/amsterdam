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
	"time"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
)

/*
 * This file defines data structures for XML vCards as defined by the XMPP Foundation specification XEP-0054, "vcard-temp",
 * https://xmpp.org/extensions/xep-0054.html.
 */

// Used to indicate a field which may be present, or not.
type Presence struct{}

// Specifies that a "present" field is "present."
var Present = &Presence{}

// The vCard version field.
const VCARD_VERSION = "2.0"

// VCard is the top level vCard structure.
type VCard struct {
	XMLName      xml.Name          `xml:"vcard-temp vCard"`      // name must be "vCard" in the "vcard-temp" namespace
	Version      string            `xml:"VERSION"`               // vCard version number
	FullName     string            `xml:"FN"`                    // full name
	Name         VCName            `xml:"N"`                     // broken-up name components
	Nickname     string            `xml:"NICKNAME,omitempty"`    // nickname (not used in Amsterdam)
	Photo        *VCPhoto          `xml:"PHOTO,omitempty"`       // user photo (not used in Amsterdam)
	BDay         string            `xml:"BDAY,omitempty"`        // birthday, ISO 8601 format
	Address      *[]VCAddress      `xml:"ADR,omitempty"`         // address
	AddressLabel *[]VCAddressLabel `xml:"LABEL,omitempty"`       // address label
	Email        *[]VCEmail        `xml:"EMAIL,omitempty"`       // E-mail address
	Tel          *[]VCTelephone    `xml:"TEL,omitempty"`         // telephone number
	JabberID     string            `xml:"JABBERID,omitempty"`    // Jabber/XMPP address (user@host) (XMPP extension) (not used in Amsterdam)
	Mailer       string            `xml:"MAILER,omitempty"`      // mailer user agent (not used in Amsterdam)
	TZ           string            `xml:"TZ,omitempty"`          // time zone indicator, ISO 8601 formatted UTC offset
	Geolocation  *VCGeolocation    `xml:"GEO,omitempty"`         // geolocation (not used in Amsterdam)
	Title        string            `xml:"TITLE,omitempty"`       // job title (not used in Amsterdam)
	Role         string            `xml:"ROLE,omitempty"`        // job role (not used in Amsterdam)
	Logo         *VCLogo           `xml:"LOGO,omitempty"`        // organization logo (not used in Amsterdam)
	Agent        *VCAgent          `xml:"AGENT,omitempty"`       // agent for the organization (not used in Amsterdam)
	Org          *VCOrganization   `xml:"ORG,omitempty"`         // organization
	Categories   *VCCategory       `xml:"CATEGORIES,omitempty"`  // categories
	Note         string            `xml:"NOTE,omitempty"`        // text note (not used by Amsterdam)
	ProductID    string            `xml:"PRODID,omitempty"`      // product ID that generated this vCard (not used by Amsterdam)
	LastUpdate   string            `xml:"REV,omitempty"`         // last update to this information, ISO 8601 format
	SortString   string            `xml:"SORT-STRING,omitempty"` // sort string
	Sound        *VCSound          `xml:"SOUND,omitempty"`       // pronunciation property (not used in Amsterdam)
	UID          string            `xml:"UID,omitempty"`         // unique identifier (not necessarily an Amsterdam UID!) (not used in Amsterdam)
	URL          string            `xml:"URL,omitempty"`         // URL
	Class        *VCClass          `xml:"CLASS,omitempty"`       // privacy classification (not used in Amsterdam)
	Key          *VCKey            `xml:"KEY,omitempty"`         // authentication credential or encryption key (not used in Amsterdam)
	Description  string            `xml:"DESC,omitempty"`        // description string value (XMPP extension)
}

// VCPhoto is the "photo" attachment to the VCard.
type VCPhoto struct {
	XMLName       xml.Name `xml:"PHOTO"`            // must be a "PHOTO" tag
	Type          string   `xml:"TYPE,omitempty"`   // data type for BINVAL
	BinaryValue   string   `xml:"BINVAL,omitempty"` // binary photo value (Base64 encoded)
	ExternalValue string   `xml:"EXTVAL,omitempty"` // external value of photo (URL)
}

// Name is the "structured name" property of the vCard.
type VCName struct {
	XMLName xml.Name `xml:"N"`                // must be an "N" tag
	Family  string   `xml:"FAMILY,omitempty"` // family name
	Given   string   `xml:"GIVEN,omitempty"`  // given name
	Middle  string   `xml:"MIDDLE,omitempty"` // middle name/initial
	Prefix  string   `xml:"PREFIX,omitempty"` // prefix
	Suffix  string   `xml:"SUFFIX,omitempty"` // suffix
}

// VCAddress is the "address" property of the vCard.
type VCAddress struct {
	XMLName       xml.Name  `xml:"ADR"`                // must be a "ADR" tag
	Work          *Presence `xml:"WORK,omitempty"`     // Presence indicates work address
	Home          *Presence `xml:"HOME,omitempty"`     // Presence indicates home address
	Postal        *Presence `xml:"POSTAL,omitempty"`   // presence indicates postal address
	Parcel        *Presence `xml:"PARCEL,omitempty"`   // presence indicates parcel address
	Domestic      *Presence `xml:"DOM,omitempty"`      // presence indicates domestic address
	International *Presence `xml:"INTL,omitempty"`     // presence indicates international address
	Preferred     *Presence `xml:"PREF,omitempty"`     // Presence indicates preferred address
	POBox         string    `xml:"POBOX,omitempty"`    // post office box
	Locality      string    `xml:"LOCALITY,omitempty"` // locality (city)
	Region        string    `xml:"REGION,omitempty"`   // region (state/province)
	PCode         string    `xml:"PCODE,omitempty"`    // postal code
	Country       string    `xml:"CTRY,omitempty"`     // country
	Street        string    `xml:"STREET,omitempty"`   // street address (addr line 1)
	ExtAddr       string    `xml:"EXTADR,omitempty"`   // extended address (addr line 2)
}

// VCAddressLabel is the "address label" property of the vCard.
type VCAddressLabel struct {
	XMLName       xml.Name  `xml:"LABEL"`            // must be a "LABEL" tag
	Work          *Presence `xml:"WORK,omitempty"`   // Presence indicates work address
	Home          *Presence `xml:"HOME,omitempty"`   // Presence indicates home address
	Postal        *Presence `xml:"POSTAL,omitempty"` // presence indicates postal address
	Parcel        *Presence `xml:"PARCEL,omitempty"` // presence indicates parcel address
	Domestic      *Presence `xml:"DOM,omitempty"`    // presence indicates domestic address
	International *Presence `xml:"INTL,omitempty"`   // presence indicates international address
	Preferred     *Presence `xml:"PREF,omitempty"`   // Presence indicates preferred address
	Lines         []string  `xml:"LINE"`             // lines of text on the address label
}

// VCEmail is the "E-mail address" property of the vCard.
type VCEmail struct {
	XMLName   xml.Name  `xml:"EMAIL"`              // must be an "EMAIL" tag
	Work      *Presence `xml:"WORK,omitempty"`     // presence indicates work address
	Home      *Presence `xml:"HOME,omitempty"`     // presence indicates home address
	Internet  *Presence `xml:"INTERNET,omitempty"` // presence indicates Internet address
	Preferred *Presence `xml:"PREF,omitempty"`     // Presence indicates preferred address
	X400      *Presence `xml:"X400,omitempty"`     // Presence indicates X.400 address
	UserID    string    `xml:"USERID"`             // user ID (address)
}

// VCTelephone is the "telephone number" property of the vCard.
type VCTelephone struct {
	XMLName   xml.Name  `xml:"TEL"`             // must be a "TEL" tag
	Work      *Presence `xml:"WORK,omitempty"`  // presence indicates work number
	Home      *Presence `xml:"HOME,omitempty"`  // presence indicates home number
	Voice     *Presence `xml:"VOICE,omitempty"` // presence indicates voice number
	Fax       *Presence `xml:"FAX,omitempty"`   // presence indicates fax number
	Pager     *Presence `xml:"PAGER,omitempty"` // presence indicates pager number
	Message   *Presence `xml:"MSG,omitempty"`   // presence indicates message number
	Cell      *Presence `xml:"CELL,omitempty"`  // presence indicates cellphone number
	Video     *Presence `xml:"VIDEO,omitempty"` // presence indicates videophone number
	BBS       *Presence `xml:"BBS,omitempty"`   // presence indicates BBS number
	Modem     *Presence `xml:"MODEM,omitempty"` // presence indicates modem number
	ISDN      *Presence `xml:"ISDN,omitempty"`  // presence indicates ISDN number
	PCS       *Presence `xml:"PCS,omitempty"`   // presence indicates PCS number
	Preferred *Presence `xml:"PREF,omitempty"`  // presence indicates preferred number
	Number    string    `xml:"NUMBER"`          // the number
}

// VCGeolocation is the "geolocation" property of the vCard.
type VCGeolocation struct {
	XMLName   xml.Name `xml:"GEO"`  // must be a "GEO" tag
	Latitude  string   `xml:"LAT"`  // latitude to six decimal places (North is positive)
	Longitude string   `xml:"LONG"` // longitude to six decimal places (East is positive)
}

// VCLogo is the "logo" property of the vCard.
type VCLogo struct {
	XMLName       xml.Name `xml:"LOGO"`             // must be a "LOGO" tag
	Type          string   `xml:"TYPE,omitempty"`   // data type for BINVAL
	BinaryValue   string   `xml:"BINVAL,omitempty"` // binary photo value (Base64 encoded)
	ExternalValue string   `xml:"EXTVAL,omitempty"` // external value of photo (URL)
}

// VCAgent is the "agent" property of the vCard.
type VCAgent struct {
	XMLName       xml.Name `xml:"AGENT"`                      // must be an "AGENT" tag
	VCard         *VCard   `xml:"vcard-temp vCard,omitempty"` // vCard with agent contact info
	ExternalValue string   `xml:"EXTVAL,omitempty"`           // external value, such as URL to contact info
}

// VCOrganization is the "organization" property of the vCard.
type VCOrganization struct {
	XMLName xml.Name  `xml:"ORG"`               // must be an "ORG" tag
	OrgName string    `xml:"ORGNAME"`           // organization name
	OrgUnit *[]string `xml:"ORGUNIT,omitempty"` // organization unit(s)
}

// VCCategory is the "category" property of the vCard.
type VCCategory struct {
	XMLName  xml.Name `xml:"CATEGORIES"` // must be a "CATEGORIES" tag
	Keywords []string `xml:"KEYWORD"`    // keywords
}

// VCSound is the "pronunciation guide" property of the vCard.
type VCSound struct {
	XMLName       xml.Name `xml:"SOUND"`              // must be a "SOUND" tag
	Phonetic      string   `xml:"PHONETIC,omitempty"` // phonetic pronunciation
	BinaryValue   string   `xml:"BINVAL,omitempty"`   // binary audio value (Base64 encoded)
	ExternalValue string   `xml:"EXTVAL,omitempty"`   // external value of audio (URL)
}

// VCClass is the "privacy classification" property of the vCard.
type VCClass struct {
	XMLName      xml.Name  `xml:"CLASS"`                  // must be a "CLASS" tag
	Public       *Presence `xml:"PUBLIC,omitempty"`       // presence indicates public information
	Private      *Presence `xml:"PRIVATE,omitempty"`      // presence indicates private information
	Confidential *Presence `xml:"CONFIDENTIAL,omitempty"` // presence indicates confidential information
}

// VCKey is the "authentication or encryption key" property of the vCard.
type VCKey struct {
	XMLName    xml.Name `xml:"KEY"`            // must be a "KEY" tag
	Type       string   `xml:"TYPE,omitempty"` // type indicator
	Credential string   `xml:"CRED"`           // credential value
}

// VCardFromContactInfo fills in a VCard structure from a ContactInfo object.
func VCardFromContactInfo(ctx context.Context, target *VCard, ci *database.ContactInfo) error {
	target.Version = VCARD_VERSION
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
	addr[0].Home = Present
	addr[0].Postal = Present
	addr[0].Preferred = Present
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
			phone[i].Home = Present
			phone[i].Voice = Present
			phone[i].Preferred = Present
			phone[i].Number = *ci.Phone
			i++
		}
		if ci.Fax != nil {
			phone[i].Home = Present
			phone[i].Fax = Present
			phone[i].Preferred = Present
			phone[i].Number = *ci.Fax
			i++
		}
		if ci.Mobile != nil {
			phone[i].Home = Present
			phone[i].Cell = Present
			phone[i].Voice = Present
			phone[i].Preferred = Present
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
		email[0].Home = Present
		email[0].Internet = Present
		email[0].Preferred = Present
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

// VCardSetContactInfo fills the ContactInfo object with data from the VCard.
func VCardSetContactInfo(ci *database.ContactInfo, vc *VCard) {
	ci.GivenName = util.IIF(vc.Name.Given == "", nil, &vc.Name.Given)
	ci.FamilyName = util.IIF(vc.Name.Family == "", nil, &vc.Name.Family)
	if vc.Name.Middle == "" {
		ci.MiddleInit = nil
	} else {
		s := vc.Name.Middle[0:1]
		ci.MiddleInit = &s
	}
	ci.Prefix = util.IIF(vc.Name.Prefix == "", nil, &vc.Name.Prefix)
	ci.Suffix = util.IIF(vc.Name.Suffix == "", nil, &vc.Name.Suffix)
	if vc.Org != nil {
		ci.Company = &(vc.Org.OrgName)
	}
	if vc.URL != "" {
		ci.URL = &(vc.URL)
	}
	addr := VCardSelectAddress(vc)
	if addr != nil {
		ci.Addr1 = util.IIF(addr.Street == "", nil, &addr.Street)
		ci.Addr2 = util.IIF(addr.ExtAddr == "", nil, &addr.ExtAddr)
		ci.Locality = util.IIF(addr.Locality == "", nil, &addr.Locality)
		ci.Region = util.IIF(addr.Region == "", nil, &addr.Region)
		ci.PostalCode = util.IIF(addr.PCode == "", nil, &addr.PCode)
		ci.Country = util.IIF(addr.Country == "", nil, &addr.Country)
	}
	email, err := VCardGetEmailAddress(vc)
	if err == nil {
		ci.Email = &email
	}
	phone, fax, mobile := VCardSelectPhones(vc)
	if phone != nil {
		ci.Phone = &(phone.Number)
	}
	if fax != nil {
		ci.Fax = &(fax.Number)
	}
	if mobile != nil {
		ci.Mobile = &(mobile.Number)
	}
}

// VCardSelectAddress selects a valid address from the VCard.
func VCardSelectAddress(vc *VCard) *VCAddress {
	if vc.Address == nil || len(*vc.Address) == 0 {
		return nil
	}
	if len(*vc.Address) == 1 {
		return &((*vc.Address)[0])
	}
	for i := range *vc.Address {
		if (*vc.Address)[i].Preferred != nil {
			return &((*vc.Address)[i])
		}
	}
	return &((*vc.Address)[0])
}

// VCardSelectPhones finds the phone, fax, and mobile numbers in the telephone block.
func VCardSelectPhones(vc *VCard) (*VCTelephone, *VCTelephone, *VCTelephone) {
	if vc.Tel == nil || len(*vc.Tel) == 0 {
		return nil, nil, nil
	}
	var mobile *VCTelephone = nil
	for i := range *vc.Tel {
		if (*vc.Tel)[i].Cell != nil {
			if mobile == nil || (*vc.Tel)[i].Preferred != nil {
				mobile = &((*vc.Tel)[i])
			}
		}
	}
	var fax *VCTelephone = nil
	for i := range *vc.Tel {
		if (*vc.Tel)[i].Fax != nil {
			if fax == nil || (*vc.Tel)[i].Preferred != nil {
				fax = &((*vc.Tel)[i])
			}
		}
	}
	var phone *VCTelephone = nil
	for i := range *vc.Tel {
		if (*vc.Tel)[i].Voice != nil && (*vc.Tel)[i].Cell == nil {
			if phone == nil || (*vc.Tel)[i].Preferred != nil {
				phone = &((*vc.Tel)[i])
			}
		}
	}
	return phone, fax, mobile
}

// VCardGetEmailAddress finds a useful E-mail address in a VCard.
func VCardGetEmailAddress(vc *VCard) (string, error) {
	if vc.Email == nil || len(*vc.Email) == 0 {
		return "", errors.New("no E-mail address found for user")
	}
	addrs := make([]*VCEmail, 0, len(*vc.Email))
	for i, a := range *vc.Email {
		if a.Internet != nil {
			addrs = append(addrs, &((*vc.Email)[i]))
		}
	}
	if len(addrs) == 0 {
		return "", errors.New("no Internet E-mail addresses found for user")
	}
	if len(addrs) == 1 {
		return addrs[0].UserID, nil
	}
	for _, a := range addrs {
		if a.Preferred != nil {
			return a.UserID, nil
		}
	}
	for _, a := range addrs {
		if a.Home != nil {
			return a.UserID, nil
		}
	}
	return addrs[0].UserID, nil
}

// VCardGetBirthday extracts the birthday from the VCard as a time value.
func VCardGetBirthday(vc *VCard) (*time.Time, error) {
	s := vc.BDay
	if s == "" {
		return nil, nil
	}
	if len(s) > 8 {
		s = s[:8]
	}
	val, err := time.Parse(ISO8601_DATE, s)
	return &val, err
}
