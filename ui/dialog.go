/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"embed"
	"fmt"
	"math"
	"net"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
	"gopkg.in/yaml.v3"
)

// DialogItemChoice holds a dialog item choice (only needed in case of defined dropdowns)
type DialogItemChoice struct {
	Id      string `yaml:"id"`
	Text    string `yaml:"text"`
	Default bool   `yaml:"default,omitempty"`
}

// DialogItem holds the dialog item definition.
type DialogItem struct {
	Type       string             `yaml:"type"`
	Name       string             `yaml:"name"`
	Caption    string             `yaml:"caption,omitempty"`
	Subcaption string             `yaml:"subcaption,omitempty"`
	Required   bool               `yaml:"required,omitempty"`
	Disabled   bool               `yaml:"disabled,omitempty"`
	Size       int                `yaml:"size,omitempty"`
	MaxLength  int                `yaml:"maxlength,omitempty"`
	Value      string             `yaml:"value,omitempty"`
	Param      string             `yaml:"param,omitempty"`
	Choices    []DialogItemChoice `yaml:"choices,omitempty"`
	AuxData    any
}

// Dialog holds the dialog definition.
type Dialog struct {
	Name         string       `yaml:"name"`
	FormName     string       `yaml:"formName"`
	Options      string       `yaml:"options,omitempty"`
	MenuSelector string       `yaml:"menuSelector,omitempty"`
	Title        string       `yaml:"title"`
	Subtitle     string       `yaml:"subtitle,omitempty"`
	Action       string       `yaml:"action"`
	Instructions string       `yaml:"instructions,omitempty"`
	Fields       []DialogItem `yaml:"fields"`
	fldmap       map[string]*DialogItem
}

// VRange is used as a return type for ValueRange.
type VRange struct {
	Low  int
	High int
}

// IsEmpty tells us if a value range is empty.
func (vr *VRange) IsEmpty() bool {
	return vr.Low == -1 && vr.High == -1
}

//go:embed dialogs/*
var dialogs embed.FS

/* AmLoadDialog loads a dialog definition.
 * Parameters:
 *     name - The name of the dialog to load
 */
func AmLoadDialog(name string) (*Dialog, error) {
	b, err := dialogs.ReadFile(fmt.Sprintf("dialogs/%s.yaml", name))
	if err == nil {
		var d Dialog
		err = yaml.Unmarshal(b, &d)
		if err == nil {
			// "nil-patch" certain fields and create the fast-lookup map
			if d.MenuSelector == "" {
				d.MenuSelector = "nochange"
			}
			d.fldmap = make(map[string]*DialogItem)
			for i, fld := range d.Fields {
				d.fldmap[fld.Name] = &(d.Fields[i])
				if fld.Type == "button" && fld.Param == "" {
					d.Fields[i].Param = "blue"
				}
				if fld.Type == "date" && fld.Param == "" {
					d.Fields[i].Param = "year:-100"
				}
				if fld.Type == "integer" && fld.Size == 0 {
					vr := fld.ValueRange()
					if !vr.IsEmpty() {
						// compute the number of digits in each end of the range and take the maximum as the size
						dlow := int(math.Floor(math.Log10(float64(vr.Low)))) + 1
						dhigh := int(math.Floor(math.Log10(float64(vr.High)))) + 1
						d.Fields[i].Size = max(dlow, dhigh)
						d.Fields[i].MaxLength = d.Fields[i].Size
					}
				}
				if fld.Type == "ipaddress" {
					d.Fields[i].Size = 15      // max IPv4
					d.Fields[i].MaxLength = 39 // max IPv6
				}
				if fld.Type == "dropdown" && len(fld.Choices) == 0 {
					return nil, fmt.Errorf("dropdown field %s in dialog %s has no choices", fld.Name, name)
				}
			}
			return &d, nil
		}
	}
	return nil, err
}

// DateValues returns the date values stored in a date field.
func (fld *DialogItem) DateValues() []int {
	if fld.Type == "date" && fld.AuxData != nil {
		return fld.AuxData.([]int)
	}
	rc := make([]int, 3)
	rc[0] = -1
	rc[1] = -1
	rc[2] = -1
	return rc
}

// IsChecked returns true if a dialog checkbox is checked.
func (fld *DialogItem) IsChecked() bool {
	if fld.Type == "checkbox" {
		return len(fld.Value) > 0
	}
	return false
}

// SetChecked sets the value of a checkbox.
func (fld *DialogItem) SetChecked(val bool) {
	if fld.Type == "checkbox" {
		if val {
			fld.Value = "Y"
		} else {
			fld.Value = ""
		}
	}
}

// ValueInt returns the value of the field as an integer.
func (fld *DialogItem) ValueInt() (int, error) {
	return strconv.Atoi(fld.Value)
}

// ValueRange returns the minimum and maximum values for an integer field.
func (fld *DialogItem) ValueRange() VRange {
	if fld.Type == "integer" && fld.Param != "" {
		parms := strings.Split(fld.Param, "-")
		low, _ := strconv.Atoi(parms[0])
		high, _ := strconv.Atoi(parms[1])
		return VRange{Low: low, High: high}
	}
	return VRange{Low: -1, High: -1}
}

// AsDate returns the value of a date field as a Go date.
func (fld *DialogItem) AsDate() *time.Time {
	if fld.Type == "date" && fld.AuxData != nil {
		v := fld.AuxData.([]int)
		if v[0] >= 1 && v[1] >= 1 && v[2] >= 1 {
			rc := time.Date(v[2], time.Month(v[0]), v[1], 0, 0, 0, 0, time.Local)
			return &rc
		}
	}
	return nil
}

// SetDate sets the value of the dialog item as a date.
func (fld *DialogItem) SetDate(d *time.Time) {
	if fld.Type == "date" {
		dvs := make([]int, 3)
		if d == nil {
			dvs[0] = -1
			dvs[1] = -1
			dvs[2] = -1
			fld.Value = ""
		} else {
			dvs[0] = int(d.Month()) - int(time.January) + 1
			dvs[1] = d.Day()
			dvs[2] = d.Year()
			fld.Value = fmt.Sprintf("%04d%02d%02d", dvs[2], dvs[0], dvs[1])
		}
		fld.AuxData = dvs
	}
}

// ValPtr returns the value of a field as a string pointer, or nil if the field is empty.
func (fld *DialogItem) ValPtr() *string {
	if fld.Value == "" {
		return nil
	}
	return &fld.Value
}

// SetVal sets the value of a field from a string pointer.
func (fld *DialogItem) SetVal(p *string) {
	fld.Value = util.SRef(p)
}

// SetInt sets the value of a field to an integer.
func (fld *DialogItem) SetInt(v int) {
	fld.Value = fmt.Sprintf("%d", v)
}

// SetLevel sets a security level into a field value.
func (fld *DialogItem) SetLevel(level uint16) {
	fld.Value = fmt.Sprintf("%d", level)
	if fld.Type == "rolelist" {
		rolelist := database.AmRoleList(fld.Param)
		fld.AuxData = rolelist.FindForLevel(level)
	}
}

// GetLevel gets a field's value as a security level.
func (fld *DialogItem) GetLevel() uint16 {
	v, err := strconv.Atoi(fld.Value)
	if err != nil {
		return uint16(0)
	}
	return uint16(v)
}

// IsEmpty returns true if the field is empty.
func (fld *DialogItem) IsEmpty() bool {
	return len(fld.Value) == 0
}

// SetTargetUser alters a dialog's content to reflect a target user.
func (d *Dialog) SetTargetUser(u *database.User) {
	d.Title = strings.ReplaceAll(d.Title, "[USERNAME]", u.Username)
	d.Subtitle = strings.ReplaceAll(d.Subtitle, "[USERNAME]", u.Username)
	d.Action = strings.ReplaceAll(d.Action, "[USERNAME]", u.Username)
	for i, fld := range d.Fields {
		switch fld.Type {
		case "userphoto", "communitylogo":
			d.Fields[i].Param = strings.ReplaceAll(fld.Param, "[USERNAME]", u.Username)
		}
	}
}

// SetCommunity alters a dialog's content to reflect the community.
func (d *Dialog) SetCommunity(comm *database.Community) {
	d.Title = strings.ReplaceAll(d.Title, "[CNAME]", comm.Name)
	d.Subtitle = strings.ReplaceAll(d.Subtitle, "[CNAME]", comm.Name)
	d.Action = strings.ReplaceAll(d.Action, "[CID]", comm.Alias)
	for i, fld := range d.Fields {
		switch fld.Type {
		case "userphoto", "communitylogo":
			d.Fields[i].Param = strings.ReplaceAll(fld.Param, "[CID]", comm.Alias)
		}
	}
}

// SetConference alters a dialog's content to reflect the conference.
func (d *Dialog) SetConference(conf *database.Conference, alias string) {
	d.Title = strings.ReplaceAll(d.Title, "[CONFNAME]", conf.Name)
	d.Subtitle = strings.ReplaceAll(d.Subtitle, "[CONFNAME]", conf.Name)
	d.Action = strings.ReplaceAll(d.Action, "[CONFID]", alias)
}

/* Field returns a pointer to a dialog's field, given its name.
 * Parameters:
 *     name - The name of the field to find.
 * Returns:
 *     Pointer to the field, or nil.
 */
func (d *Dialog) Field(name string) *DialogItem {
	return d.fldmap[name]
}

/* Render sets up the rendering parameters to send this dialog to the output.
 * Parameters:
 *     ctxt - The AmContext for this request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func (d *Dialog) Render(ctxt AmContext) (string, any) {
	required := false
	for i, fld := range d.Fields {
		if fld.Required {
			required = true // display the "required" blurb
		}
		switch fld.Type {
		case "password": // clear all "password" fields as a security measure
			d.Fields[i].Value = ""
		case "localelist": // default locale if we don't have one
			if d.Fields[i].Value == "" {
				d.Fields[i].Value = config.GlobalConfig.Defaults.Language
			}
		case "tzlist": // default timezone if we don't have any
			if d.Fields[i].Value == "" {
				d.Fields[i].Value = config.GlobalConfig.Defaults.TimeZone
			}
		case "dropdown":
			defv := ""
			for _, ch := range fld.Choices {
				if ch.Default {
					defv = ch.Id
					break
				}
			}
			if d.Fields[i].Value == "" {
				d.Fields[i].Value = defv
			} else {
				ok := false
				for _, ch := range fld.Choices {
					if d.Fields[i].Value == ch.Id {
						ok = true
						break
					}
				}
				if !ok {
					d.Fields[i].Value = defv
				}
			}
		case "rolelist":
			if d.Fields[i].AuxData == nil {
				rolelist := database.AmRoleList(fld.Param)
				role := rolelist.FindForLevel(d.Fields[i].GetLevel())
				if role == nil {
					role := rolelist.Default()
					d.Fields[i].Value = role.LevelStr()
				}
				d.Fields[i].AuxData = role
			}
		}
	}
	if d.MenuSelector != "" && d.MenuSelector != "nochange" {
		ctxt.SetLeftMenu(d.MenuSelector)
	}
	ctxt.VarMap().Set("__required", required)
	ctxt.VarMap().Set("__dialog", d)
	ctxt.SetFrameTitle(d.Title)
	if strings.Contains(d.Options, "suppresslogin") {
		ctxt.SetScratch("frame_suppressLogin", true)
	}
	return "framed", "dialog.jet"
}

/* RenderError sets up the rendering parameters to send this dialog to the output with an error message.
 * Parameters:
 *     ctxt - The AmContext for this request.
 *     errormessage - The error message to be displayed.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func (d *Dialog) RenderError(ctxt AmContext, errormessage string) (string, any) {
	ctxt.VarMap().Set("__errorMessage", errormessage)
	return d.Render(ctxt)
}

/* RenderInfo sets up the rendering parameters to send this dialog to the output with an info message.
 * Parameters:
 *     ctxt - The AmContext for this request.
 *     infoMessage - The info message to be displayed.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func (d *Dialog) RenderInfo(ctxt AmContext, infoMessage string) (string, any) {
	ctxt.VarMap().Set("__infoMessage", infoMessage)
	return d.Render(ctxt)
}

/* LoadFromForm loads the values in a dialog from the form fields in the request.
 * Parameters:
 *     ctxt - The AmContext for this request.
 */
func (d *Dialog) LoadFromForm(ctxt AmContext) {
	for i, fld := range d.Fields {
		d.Fields[i].Value = ""
		switch fld.Type {
		case "header":
			continue
		case "button":
			continue
		case "date":
			dvals := make([]int, 3)
			var err error
			dvals[0], err = ctxt.FormFieldInt(fmt.Sprintf("%s_month", fld.Name))
			if err != nil {
				dvals[0] = -1
				d.Fields[i].Value = fmt.Sprintf("!undefined month %s: %v", fld.Name, err)
			}
			dvals[1], err = ctxt.FormFieldInt(fmt.Sprintf("%s_day", fld.Name))
			if err != nil {
				dvals[1] = -1
				if d.Fields[i].Value == "" {
					d.Fields[i].Value = fmt.Sprintf("!undefined day %s: %v", fld.Name, err)
				}
			}
			dvals[2], err = ctxt.FormFieldInt(fmt.Sprintf("%s_year", fld.Name))
			if err != nil {
				dvals[2] = -1
				if d.Fields[i].Value == "" {
					d.Fields[i].Value = fmt.Sprintf("!undefined year %s: %v", fld.Name, err)
				}
			}
			if dvals[0] > 0 && dvals[1] > 0 && dvals[2] > 0 {
				d.Fields[i].Value = fmt.Sprintf("%04d%02d%02d", dvals[2], dvals[0], dvals[1])
			} else if d.Fields[i].Value == "" && fld.Required {
				if dvals[0] <= 0 {
					d.Fields[i].Value = fmt.Sprintf("!month not set %s", fld.Name)
				} else if dvals[1] <= 0 {
					d.Fields[i].Value = fmt.Sprintf("!day not set %s", fld.Name)
				} else if dvals[2] <= 0 {
					d.Fields[i].Value = fmt.Sprintf("!year not set %s", fld.Name)
				}
			}
			d.Fields[i].AuxData = dvals
		case "userphoto", "communitylogo":
			d.Fields[i].Value = ctxt.FormField(fmt.Sprintf("%s_data", fld.Name))
		case "rolelist":
			d.Fields[i].Value = ctxt.FormField(fld.Name)
			rolelist := database.AmRoleList(d.Fields[i].Param)
			role := rolelist.FindForLevel(d.Fields[i].GetLevel())
			if role != nil {
				d.Fields[i].AuxData = role
			}
		default:
			d.Fields[i].Value = ctxt.FormField(fld.Name)
		}
	}
}

// Values returns all a dialog's values as a map.
func (d *Dialog) Values() map[string]string {
	rc := map[string]string{}
	for _, fld := range d.Fields {
		rc[fld.Name] = fld.Value
	}
	return rc
}

/* WhichButton returns an indication of which button on the dialog was clicked.
 * Parameters:
 *     ctxt - The AmContext associated with the request.
 * Returns:
 *     The name of the button field on this dialog that was clicked. If none were, the empty string is returned.
 */
func (d *Dialog) WhichButton(ctxt AmContext) string {
	for _, fld := range d.Fields {
		if fld.Type == "button" && ctxt.FormFieldIsSet(fld.Name) {
			return fld.Name
		}
	}
	return ""
}

// validatorFunc is a function that validates the contents of a dialog item.
type validatorFunc func(*DialogItem) error

// nilValidator is a validator function that doesn't do anything.
func nilValidator(*DialogItem) error {
	return nil
}

/* validateTextField validates a text field.
 * Parameters:
 *     fld - The field to be validated.
 * Returns:
 *     Standard Go error status.
 */
func validateTextField(fld *DialogItem) error {
	if len(fld.Value) == 0 && fld.Required {
		return fmt.Errorf("value of field \"%s\" is required", fld.Caption)
	}
	if len(fld.Value) > fld.MaxLength {
		return fmt.Errorf("value of field \"%s\" can be no longer than %d characters", fld.Caption, fld.MaxLength)
	}
	return nil
}

/* validateIntegerField validates an integer field.
 * Parameters:
 *     fld - The field to be validated.
 * Returns:
 *     Standard Go error status.
 */
func validateIntegerField(fld *DialogItem) error {
	err := validateTextField(fld)
	if err == nil {
		var v int
		v, err = strconv.Atoi(fld.Value)
		if err == nil {
			fld.AuxData = v // cache parsed value
			vr := fld.ValueRange()
			if vr.Low != -1 && vr.High != -1 {
				if v < vr.Low {
					return fmt.Errorf("value of field \"%s\" cannot be less than %d", fld.Caption, vr.Low)
				} else if v > vr.High {
					return fmt.Errorf("value of field \"%s\" cannot be greater than %d", fld.Caption, vr.High)
				}
			}
		} else {
			return fmt.Errorf("value of field \"%s\" is not a valid integer", fld.Caption)
		}
	}
	return nil
}

/* validateAmsIdField validates an Amsterdam ID field.
 * Parameters:
 *     fld - The field to be validated.
 * Returns:
 *     Standard Go error status.
 */
func validateAmsIdField(fld *DialogItem) error {
	err := validateTextField(fld)
	if err == nil {
		if !database.AmIsValidAmsterdamID(fld.Value) {
			err = fmt.Errorf("value of field \"%s\" is not a valid identifier", fld.Caption)
		}
	}
	return err
}

/* validateEmailField validates an E-mail address field.
 * Parameters:
 *     fld - The field to be validated.
 * Returns:
 *     Standard Go error status.
 */
func validateEmailField(fld *DialogItem) error {
	err := validateTextField(fld)
	if err == nil {
		_, err = mail.ParseAddress(fld.Value)
	}
	return err
}

/* validateIPAddressField validates an IP address field.
 * Parameters:
 *     fld - The field to be validated.
 * Returns:
 *     Standard Go error status.
 */
func validateIPAddressField(fld *DialogItem) error {
	err := validateTextField(fld)
	if err == nil {
		if strings.Contains(fld.Param, "mask") {
			// look for a CIDR mask value like "/24"
			var ok bool
			ok, err = regexp.Match("^/[0-9]+$", []byte(fld.Value))
			if err == nil {
				if ok {
					return nil // found it!
				}
			}
		}
		if err == nil {
			ip := net.ParseIP(fld.Value)
			if ip == nil {
				err = fmt.Errorf("value of field \"%s\" is not a valid IP address", fld.Caption)
			}
		}
	}
	return err
}

/* validateCountryField validates a country code field.
 * Parameters:
 *     fld - The field to be validated.
 * Returns:
 *     Standard Go error status.
 */
func validateCountryField(fld *DialogItem) error {
	if fld.Value == "XX" && fld.Required {
		return fmt.Errorf("country field \"%s\" not set", fld.Caption)
	}
	return nil
}

/* validateRoleListField validates a role list field.
 * Parameters:
 *     fld - The field to be validated.
 * Returns:
 *     Standard Go error status.
 */
func validateRoleListField(fld *DialogItem) error {
	if fld.AuxData == nil {
		return fmt.Errorf("invalid role level %s found in field \"%s\"", fld.Value, fld.Caption)
	}
	return nil
}

/* validateDateField validates a date field.
 * Parameters:
 *     fld - The field to be validated.
 * Returns:
 *     Standard Go error status.
 */
func validateDateField(fld *DialogItem) error {
	if len(fld.Value) == 0 && fld.Required {
		return fmt.Errorf("date value %s not set", fld.Caption)
	}
	if fld.Value[0] == '!' {
		return fmt.Errorf("date value %s erroneous: %s", fld.Caption, fld.Value[1:])
	}
	if fld.AuxData == nil {
		return fmt.Errorf("date value %s not set properly", fld.Caption)
	}
	dv := fld.AuxData.([]int)
	if dv[0] > 12 || dv[1] > 31 {
		return fmt.Errorf("date value %s malformed", fld.Caption)
	}
	q := fmt.Sprintf("%04d%02d%02d", dv[2], dv[0], dv[1])
	if q != fld.Value {
		return fmt.Errorf("date value %s should be %s but is %s", fld.Caption, q, fld.Value)
	}
	return nil
}

// validators maps the field types to validator functions.
var validators = map[string]validatorFunc{
	"ams_id":        validateAmsIdField,
	"button":        nilValidator,
	"checkbox":      nilValidator,
	"communitylogo": nilValidator,
	"countrylist":   validateCountryField,
	"date":          validateDateField,
	"dropdown":      nilValidator,
	"email":         validateEmailField,
	"header":        nilValidator,
	"hidden":        nilValidator,
	"integer":       validateIntegerField,
	"ipaddress":     validateIPAddressField,
	"localelist":    nilValidator,
	"password":      validateTextField,
	"rolelist":      validateRoleListField,
	"text":          validateTextField,
	"tzlist":        nilValidator,
	"userphoto":     nilValidator,
}

/* Validate validates the values in the dialog.
 * Returns:
 *     Standard Go error status.
 */
func (d *Dialog) Validate() error {
	for i, fld := range d.Fields {
		if len(fld.Value) > 0 || fld.Required {
			vfunc := validators[fld.Type]
			if vfunc != nil {
				err := vfunc(&(d.Fields[i]))
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("don't know how to validate field %s of type %s", fld.Name, fld.Type)
			}
		}
	}
	return nil
}
