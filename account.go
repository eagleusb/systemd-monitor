package main

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/smtp"
	"time"

	"github.com/nhooyr/color/log"
	"github.com/pelletier/go-toml"
)

type account struct {
	username     string
	host         string
	addr         string
	c            *smtp.Client
	a            smtp.Auth
	msg          *message
	destinations []string
	last         time.Time
	backup       *account
}

func (a *account) init(tree *toml.TomlTree) {
	a.username = necessary(tree, "username")
	a.addr = necessary(tree, "addr")
	password := optional(tree, "password")

	var err error
	a.host, _, err = net.SplitHostPort(a.addr)
	if err != nil {
		log.Fatalf("%s: addr is not in %q format", pos(tree, "addr"), "host:port")
	}
	a.a = smtp.PlainAuth("", a.username, password, a.host)
	if err = a.dial(); err != nil {
		log.Print(err)
	}
	v := tree.Get("destinations")
	if v == nil {
		log.Fatalf("%s: no %q table of arrays", pos(tree, ""), "destinations")
	}
	trees, ok := v.([]*toml.TomlTree)
	if !ok {
		log.Fatalf("%s: %q is not a table of arrays", pos(tree, "destinations"), "destinations")
	}

	a.msg = &message{buf: make([]byte, 0, 3000)}
	a.msg.write("From: ")
	a.msg.writeEmail(optional(tree, "name"), a.username)
	a.msg.write("\r\nContent-Type: text/plain; charset=UTF-8\r\nTo:")
	a.destinations = make([]string, len(trees))
	for i, tree := range trees {
		name := optional(tree, "name")
		email := necessary(tree, "email")
		a.destinations[i] = email
		a.msg.writeByte(' ')
		a.msg.writeEmail(name, email)
		if i != len(trees)-1 {
			a.msg.writeByte(',')
		}
		a.msg.write("\r\n")
	}
	a.msg.write("Subject: ")
	a.msg.initialized()

	v = tree.Get("backup")
	if v == nil {
		return
	}
	tree, ok = v.(*toml.TomlTree)
	if !ok {
		log.Fatalf("%s: %q is not a table", pos(tree, "backup"), "backup")
	}
	a.backup = new(account)
	a.backup.init(tree)
}

var errTimeout = errors.New("reconnection timeout")

func (a *account) dial() (err error) {
	now := time.Now()
	if now.Sub(a.last) < time.Second*30 {
		return errTimeout
	}
	a.last = now
	a.c, err = smtp.Dial(a.addr)
	if err != nil {
		return
	}
	var ok bool
	if ok, _ = a.c.Extension("STARTTLS"); ok {
		if err = a.c.StartTLS(&tls.Config{ServerName: a.host}); err != nil {
			return
		}
	}
	if ok, _ = a.c.Extension("AUTH"); ok && a.a != nil {
		if err = a.c.Auth(a.a); err != nil {
			return
		}
	}
	return
}

func (a *account) send(subject string, body []byte) {
	log.Printf("%s: sending emails", a.username)
	if err := a.mail(subject, body); err != nil {
		if err == io.EOF {
			log.Printf("%s: reconnecting", a.username)
			if err = a.dial(); err == nil {
				a.send(subject, body)
				return
			}
		}
		log.Printf("%s: error: %s", a.username, err)
		if a.backup != nil {
			log.Printf("%s: falling back to: %s", a.username, a.backup.username)
			a.backup.send(subject, body)
		}
		return
	}
	log.Printf("%s: sent emails", a.username)
}

func (a *account) mail(subject string, body []byte) (err error) {
	if a.c == nil {
		log.Printf("%s: reconnecting", a.username)
		if err = a.dial(); err != nil {
			return
		}
	}
	defer a.msg.reset()
	a.msg.write(subject)
	a.msg.write("\r\n\r\n")
	a.msg.writeBytes(body)
	if err = a.c.Mail(a.username); err != nil {
		return
	}
	for _, addr := range a.destinations {
		if err = a.c.Rcpt(addr); err != nil {
			return
		}
	}
	w, err := a.c.Data()
	if err != nil {
		return
	}
	_, err = w.Write(a.msg.buf)
	if err != nil {
		return
	}
	return w.Close()
}
