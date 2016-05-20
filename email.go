package main

import (
	"crypto/tls"
	"encoding/json"

	"github.com/nhooyr/color/log"

	"gopkg.in/gomail.v2"
)

type email struct {
	Name               string `json:"name"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	Destination        string `json:"destination"`
	Backup             *email `json:"backup"`
	m                  *gomail.Message
	d                  *gomail.Dialer
}

func (e *email) init(from string) {
	e.d = new(gomail.Dialer)
	if e.Destination == "" {
		b, err := json.Marshal(e)
		if err != nil {
			log.Fatal(err)
		}
		log.Fatalf("empty Destination\n%s", b)
	}
	if e.Host == "" {
		b, err := json.Marshal(e)
		if err != nil {
			log.Fatal(err)
		}
		log.Fatalf("empty Host\n%s", b)
	}
	e.d.Host = e.Host
	e.d.Port = e.Port
	e.d.Username = e.Username
	e.d.Password = e.Password
	if e.InsecureSkipVerify {
		e.d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	e.m = gomail.NewMessage()
	e.m.SetHeader("From", from)
	e.m.SetHeader("To", (&gomail.Message{}).FormatAddress(e.Destination, e.Name))
	if e.Backup != nil {
		e.Backup.init(from)
	}
}

func (e *email) send(subject, body string) {
	e.m.SetHeader("Subject", subject)
	e.m.SetBody("text/plain; charset=UTF-8", body)
	if err := e.d.DialAndSend(e.m); err != nil {
		log.Printf("error sending to %s: %s", e.Destination, err)
		if e.Backup != nil {
			log.Printf("sending to backup of %s: %s", e.Destination, e.Backup.Destination)
			e.Backup.send(subject, body)
		}
		return
	}
	log.Printf("sent email to %s", e.Destination)
}
