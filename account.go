package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"time"

	"github.com/pelletier/go-toml"
)

type account struct {
	username     string
	host         string
	addr         string
	c            *smtp.Client
	a            smtp.Auth
	msg          []byte
	mlen         int
	destinations []string
	last         time.Time
	backup       *account
}

func necessary(tree *toml.TomlTree, key string) string {
	v := tree.Get(key)
	if v == nil {
		log.Fatalf("%s: no %q key", pos(tree, ""), key)
	}
	s, ok := v.(string)
	if !ok {
		log.Fatalf("%s: %q is not a string", pos(tree, key), key)
	}
	return s
}

func optional(tree *toml.TomlTree, key string) string {
	s, ok := tree.GetDefault(key, "").(string)
	if !ok {
		log.Fatalf("%s: %q is not a string", pos(tree, key), key)
	}
	return s
}

var errTimeout = errors.New("reconnection timeout")

func (a *account) dial() (err error) {
	if time.Since(a.last) < time.Second*30 {
		return errTimeout
	}
	a.last = time.Now()
	a.c, err = smtp.Dial(a.addr)
	if err != nil {
		return err
	}
	var ok bool
	if ok, _ = a.c.Extension("STARTTLS"); ok {
		if err = a.c.StartTLS(&tls.Config{ServerName: a.host}); err != nil {
			return err
		}
	}
	if ok, _ = a.c.Extension("AUTH"); ok && a.a != nil {
		if err = a.c.Auth(a.a); err != nil {
			return err
		}
	}
	return
}

func (a *account) initMsg(tree *toml.TomlTree) {
	v := tree.Get("destinations")
	if v == nil {
		log.Fatalf("%s: no %q table", pos(tree, ""), "destinations")
	}
	trees, ok := v.([]*toml.TomlTree)
	if !ok {
		log.Fatalf("%s: %q is not a table", pos(tree, "destinations"), "destinations")
	}

	a.msg = make([]byte, 0, 3000)
	a.msg = append(a.msg, "From: "...)
	if name := optional(tree, "name"); name == "" {
		a.msg = append(a.msg, a.username...)
	} else {
		a.msg = append(a.msg, name...)
		a.msg = append(a.msg, " <"...)
		a.msg = append(a.msg, a.username...)
		a.msg = append(a.msg, '>')
	}
	a.msg = append(a.msg, "\r\nContent-Type: text/plain; charset=UTF-8\r\nTo:"...)
	a.destinations = make([]string, len(trees))
	for i, tree := range trees {
		name := optional(tree, "name")
		email := necessary(tree, "email")
		a.destinations[i] = email
		a.msg = append(a.msg, ' ')
		if name == "" {
			a.msg = append(a.msg, email...)
		} else {
			a.msg = append(a.msg, name...)
			a.msg = append(a.msg, " <"...)
			a.msg = append(a.msg, email...)
			a.msg = append(a.msg, '>')
		}
		if i != len(trees)-1 {
			a.msg = append(a.msg, ',')
		}
		a.msg = append(a.msg, "\r\n"...)
	}
	a.msg = append(a.msg, "Subject: "...)
	a.mlen = len(a.msg)

}

func (a *account) init(tree *toml.TomlTree) {
	a.username = necessary(tree, "username")
	a.addr = necessary(tree, "addr")
	password := optional(tree, "password")

	var err error
	a.host, _, err = net.SplitHostPort(a.addr)
	if err != nil {
		log.Fatalf("%s: addr must be host:port", pos(tree, "addr"))
	}
	a.a = smtp.PlainAuth("", a.username, password, a.host)
	if err = a.dial(); err != nil {
		log.Print(err)
	}
	a.initMsg(tree)

	v := tree.Get("backup")
	if v == nil {
		return
	}
	var ok bool
	tree, ok = v.(*toml.TomlTree)
	if !ok {
		log.Fatalf("%s: %q is not a table", pos(tree, "backup"), "backup")
	}
	a.backup = new(account)
	a.backup.init(tree)
}

func pos(tree *toml.TomlTree, key string) string {
	p := tree.GetPosition(key)
	return fmt.Sprintf("pos %dl %dc", p.Line, p.Col)
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
	a.msg = a.msg[:a.mlen]
	a.msg = append(a.msg, subject...)
	a.msg = append(a.msg, "\r\n\r\n"...)
	a.msg = append(a.msg, body...)
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
	_, err = w.Write(a.msg)
	if err != nil {
		return
	}
	return w.Close()
}
