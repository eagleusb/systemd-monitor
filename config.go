package main

import (
	"fmt"
	"net"
	"net/smtp"

	"github.com/nhooyr/color/log"
	"github.com/pelletier/go-toml"
)

var accounts []*email

func init(path string) {
	tree, err := toml.LoadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range c.Emails {
		if e.To == "" {
			log.Fatal("empty to")
		}
		if e.Addr == "" {
			e.fatal("empty address")
		}
		var err error
		e.d, err = smtp.Dial(e.Addr)
		if err != nil {
			e.fatal(err)
		}
		e.msg = []byte(fmt.Sprintf("From: %s <%s>\r\n", e.From, e.Username) +
			fmt.Sprintf("To: %s <%s>\r\n", e.Name, e.To) +
			"Subject: ")
		host, _, err := net.SplitHostPort(e.Addr)
		if err != nil {
			log.Fatal(err)
		}
		err = e.d.Auth(smtp.PlainAuth("", e.Username, e.Password, host))
		if err != nil {
			log.Fatal(err)
		}
		if e.Backup != nil {
			e.Backup.init()
		}
	}
}
