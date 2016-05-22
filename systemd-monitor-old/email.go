package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"

	"github.com/nhooyr/color/log"
)

type account struct {
	username string
	password string
	to       []*destination
	backup   *email
	d        *smtp.Client
	a        smtp.Auth
	msg      []byte
}

type destination struct {
	Name  string
	Email string
}

func (e *email) fatal(err error) {
	log.Fatalf("%s\n%s", err, b)
}

func (e *email) send(subject string, body []byte) {
	if err := e.sendMail(subject, body); err != nil {
		log.Printf("error sending to %s: %s", e.Destination, err)
		if e.Backup != nil {
			log.Printf("sending to backup of %s: %s", e.Destination, e.Backup.Destination)
			e.Backup.send(subject, body)
		}
		return
	}
	log.Printf("sent email to %s", e.Destination)
}

func (e *email) sendMail(subject string, body []byte) error {
	msg := append(e.msg, subject...)
	msg = append(e.msg, body...)
	var ok bool
	if ok, _ = e.d.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{ServerName: c.serverName}); err != nil {
			return err
		}
	}
	if ok, _ = e.d.Extension("AUTH"); ok && e.a != nil {
		if err = e.d.Auth(e.a); err != nil {
			return err
		}
	}
	if err = e.d.Mail(e.Username); err != nil {
		return err
	}
	for _, addr := range e.To {
		if err = e.d.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := e.d.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	return err
}
