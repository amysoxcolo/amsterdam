/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
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
	"net/mail"
	"strconv"
	"strings"
	"time"

	"git.erbosoft.com/amy/amsterdam/database"
	"gopkg.in/yaml.v3"
)

// DialogItem holds the dialog item definition.
type DialogItem struct {
	Type       string `yaml:"type"`
	Name       string `yaml:"name"`
	Caption    string `yaml:"caption,omitempty"`
	Subcaption string `yaml:"subcaption,omitempty"`
	Required   bool   `yaml:"required,omitempty"`
	Size       int    `yaml:"size,omitempty"`
	MaxLength  int    `yaml:"maxlength,omitempty"`
	Value      string `yaml:"value,omitempty"`
	Param      string `yaml:"param,omitempty"`
	AuxData    any
}

// Dialog holds the dialog definition.
type Dialog struct {
	Name         string       `yaml:"name"`
	FormName     string       `yaml:"formName"`
	Options      string       `yaml:"options,omitempty"`
	MenuSelector string       `yaml:"menuSelector,omitempty"`
	Title        string       `yaml:"title"`
	Action       string       `yaml:"action"`
	Instructions string       `yaml:"instructions,omitempty"`
	Fields       []DialogItem `yaml:"fields"`
}

// VRange is used as a return type for ValueRange.
type VRange struct {
	Low  int
	High int
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
			// "nil-patch" certain fields
			if d.MenuSelector == "" {
				d.MenuSelector = "nochange"
			}
			for i, fld := range d.Fields {
				if fld.Type == "button" && fld.Param == "" {
					d.Fields[i].Param = "blue"
				}
				if fld.Type == "date" && fld.Param == "" {
					d.Fields[i].Param = "year:-100"
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
			rc := time.Date(v[2], time.Month(v[0]), v[1], 0, 0, 0, 0, time.Now().Location())
			return &rc
		}
	}
	return nil
}

// ValPtr returns the value of a field as a string pointer, or nil if the field is empty.
func (fld *DialogItem) ValPtr() *string {
	if fld.Value == "" {
		return nil
	}
	return &fld.Value
}

/* Field returns a pointer to a dialog's field, given its name.
 * Parameters:
 *     name - The name of the field to find.
 * Returns:
 *     Pointer to the field, or nil.
 */
func (d *Dialog) Field(name string) *DialogItem {
	for i, f := range d.Fields {
		if f.Name == name {
			return &(d.Fields[i])
		}
	}
	return nil
}

/* Render sets up the rendering parameters to send this dialog to the output.
 * Parameters:
 *     ctxt - The AmContext for this request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func (d *Dialog) Render(ctxt AmContext) (string, any, error) {
	required := false
	for i, fld := range d.Fields {
		if fld.Required {
			required = true // display the "required" blurb
		}
		switch fld.Type {
		case "password": // clear all "password" fields as a security measure
			d.Fields[i].Value = ""
		case "localelist": // default locale to en-US if we don't have one
			if d.Fields[i].Value == "" {
				d.Fields[i].Value = "en-US"
			}
		}
	}
	ctxt.VarMap().Set("amsterdam_required", required)
	ctxt.VarMap().Set("amsterdam_dialog", d)
	ctxt.VarMap().Set("amsterdam_pageTitle", d.Title)
	if strings.Contains(d.Options, "suppresslogin") {
		ctxt.VarMap().Set("amsterdam_suppressLogin", true)
	}
	return "framed_template", "dialog.jet", nil
}

/* RenderError sets up the rendering parameters to send this dialog to the output with an error message.
 * Parameters:
 *     ctxt - The AmContext for this request.
 *     errormessage - The error message to be displayed.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func (d *Dialog) RenderError(ctxt AmContext, errormessage string) (string, any, error) {
	ctxt.VarMap().Set("amsterdam_errorMessage", errormessage)
	return d.Render(ctxt)
}

/* RenderInfo sets up the rendering parameters to send this dialog to the output with an info message.
 * Parameters:
 *     ctxt - The AmContext for this request.
 *     infoMessage - The info message to be displayed.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func (d *Dialog) RenderInfo(ctxt AmContext, infoMessage string) (string, any, error) {
	ctxt.VarMap().Set("amsterdam_infoMessage", infoMessage)
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
		case "userphoto":
			d.Fields[i].Value = ctxt.FormField(fmt.Sprintf("%s_data", fld.Name))
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
	"ams_id":      validateAmsIdField,
	"button":      nilValidator,
	"checkbox":    nilValidator,
	"countrylist": validateCountryField,
	"date":        validateDateField,
	"email":       validateEmailField,
	"header":      nilValidator,
	"hidden":      nilValidator,
	"integer":     validateIntegerField,
	"localelist":  nilValidator, // TODO
	"password":    validateTextField,
	"text":        validateTextField,
	"tzlist":      nilValidator, // TODO
	"userphoto":   nilValidator,
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
