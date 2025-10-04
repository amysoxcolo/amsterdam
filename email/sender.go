/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package email contains support for E-mail messages sent by Amsterdam.
package email

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"maps"
	"net/smtp"
	"os"
	"slices"
	"strings"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/CloudyKit/jet/v6"
	"github.com/CloudyKit/jet/v6/loaders/embedfs"
	log "github.com/sirupsen/logrus"
)

//go:embed templates/*
var emailTemplates embed.FS

// email_renderer is a separate Jet instance for making E-mail messages.
var emailRenderer *jet.Set

// disclaimerLines is the disclaimer from the configuration broken into lines.
var disclaimerLines []string

// signatureLines is the signature from the configuration broken into lines.
var signatureLines []string

// The mail host and port.
var mailHost string

// The SMTP authentication to use.
var auth smtp.Auth

// formatMessage takes a message and turns it into serialized bytes for sending.
func formatMessage(m *amMessage) ([]byte, error) {
	if m.template != "" {
		// Render the template for the message, which may reset Subject.
		templ, err := emailRenderer.GetTemplate(m.template)
		if err == nil {
			var buf bytes.Buffer
			err = templ.Execute(&buf, m.vars, Message(m))
			if err == nil {
				m.text = buf.String()
			}
		}
		if err != nil {
			return make([]byte, 0), err
		}
	}
	user, err := database.AmGetUser(m.uid)
	if err == nil {
		// Build the final headers.
		hdrs := make(map[string]string)
		maps.Copy(hdrs, m.headers)
		hdrs["From"] = m.from
		hdrs["To"] = strings.Join(m.to, ", ")
		hdrs["Cc"] = strings.Join(m.cc, ", ")
		hdrs["Bcc"] = strings.Join(m.bcc, ", ")
		hdrs["Subject"] = m.subject
		hdrs["Content-Type"] = "text/plain; charset=UTF-8"
		me, _ := os.Hostname()
		hdrs["X-Amsterdam-Server-Info"] = fmt.Sprintf("%s (Amsterdam/%s)", me, config.AMSTERDAM_VERSION)
		hdrs["X-Amsterdam-Sender-Info"] = fmt.Sprintf("uid %d, name %s, ip [%s]", m.uid, user.Username, m.ip)
		for i, v := range disclaimerLines {
			hdrs[fmt.Sprintf("X-Disclaimer-%d", i)] = v
		}

		// Sort the header keys tro make for a better presentation.
		keys := make([]string, 0, len(hdrs))
		for k := range hdrs {
			keys = append(keys, k)
		}
		slices.Sort(keys)

		// Build the actual message.
		var out bytes.Buffer
		for _, k := range keys {
			fmt.Fprintf(&out, "%s: %s\r\n", k, hdrs[k])
		}
		out.WriteString("\r\n")
		for _, l := range strings.Split(m.text, "\n") {
			fmt.Fprintf(&out, "%s\r\n", l)
		}
		out.WriteString("--\r\n")
		for _, l := range signatureLines {
			fmt.Fprintf(&out, "%s\r\n", l)
		}
		return out.Bytes(), nil
	}
	return make([]byte, 0), err
}

// transmitMessage handles the sending of the message.
func transmitMessage(m *amMessage, body []byte) {
	cl, err := smtp.Dial(mailHost)
	if err == nil {
		defer cl.Close()
		me, _ := os.Hostname()
		if err = cl.Hello(me); err == nil {
			if config.GlobalConfig.Email.Tls == "starttls" {
				if ok, _ := cl.Extension("STARTTLS"); ok {
					err = cl.StartTLS(nil)
				} else {
					log.Infof("server %s does not support STARTTLS", mailHost)
				}
			}
			if err == nil {
				if err = cl.Auth(auth); err == nil {
					if err = cl.Mail(m.fromAddr); err == nil {
						for _, addr := range m.toAddrs {
							if err = cl.Rcpt(addr); err != nil {
								log.Errorf("failed to set recipient address: %v", err)
								break
							}
						}
						if err == nil {
							var w io.WriteCloser
							w, err = cl.Data()
							if err == nil {
								_, err = w.Write(body)
								if err != nil {
									log.Errorf("failed to write message data: %v", err)
								}
								err = w.Close()
								if err != nil {
									log.Errorf("failed to close and send: %v", err)
								}
								err = cl.Quit()
								if err != nil {
									log.Errorf("failed to quit session: %v", err)
								}
							} else {
								log.Errorf("failed to start writing data: %v", err)
							}
						}
					} else {
						log.Errorf("failed to set sender: %v", err)
					}
				} else {
					log.Errorf("failed to authenticate to server: %v", err)
				}
			} else {
				log.Errorf("failed to start TLS handshake: %v", err)
			}
		} else {
			log.Errorf("error sending HELO to server: %v", err)
		}
	} else {
		log.Errorf("unable to contact host %s via SMTP: %v", mailHost, err)
	}
}

// senderLoop collects E-mail messages from the channel and pushes them out.
func senderLoop(sent chan *amMessage, done chan bool) {
	for m := range sent {
		body, err := formatMessage(m)
		if err == nil {
			transmitMessage(m, body)
		} else {
			log.Errorf("unable to format message: %v", err)
		}
		go recycleMessage(m)
	}
	done <- true // signal done for synchronization
}

// sendChan is the channel we put E-mail messages on to be sent.
var sendChan chan *amMessage

// doneChan is the channel that gets signaled when the senderLoop breaks.
var doneChan chan bool

// SetupMailSender starts the mail-sending goroutine.
func SetupMailSender() func() {
	// Initialize mail host and authentication.
	mailHost = fmt.Sprintf("%s:%d", config.GlobalConfig.Email.Host, config.GlobalConfig.Email.Port)
	switch config.GlobalConfig.Email.AuthType {
	case "plain":
		auth = smtp.PlainAuth("", config.GlobalConfig.Email.User, config.GlobalConfig.Email.Password,
			config.GlobalConfig.Email.Host)
	default:
		panic("Unknown auth type: " + config.GlobalConfig.Email.AuthType)
	}

	// Split the configured disclaimer and signature.
	disclaimerLines = strings.Split(config.GlobalConfig.Email.Disclaimer, "\n")
	signatureLines = strings.Split(config.GlobalConfig.Email.Signature, "\n")

	// Initialize the template engine.
	emailRenderer = jet.NewSet(
		embedfs.NewLoader("templates/", emailTemplates),
		jet.DevelopmentMode(true),
	)

	// Start the sender loop.
	sendChan = make(chan *amMessage, 16)
	doneChan = make(chan bool)
	go senderLoop(sendChan, doneChan)
	return func() {
		close(sendChan) // will break the loop in senderLoop
		<-doneChan      // wait for routine to complete
	}
}
