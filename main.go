package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/smtp"
	"os/exec"
	"strings"
	"sync"

	"github.com/nhooyr/color/log"
	"github.com/pelletier/go-toml"
)

type account struct {
	username     string
	host         string
	c            *smtp.Client
	a            smtp.Auth
	msg          []byte
	mlen         int
	destinations []string
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

func (a *account) init(tree *toml.TomlTree) {
	a.username = necessary(tree, "username")
	name := optional(tree, "name")
	addr := necessary(tree, "addr")
	password := optional(tree, "password")

	var err error
	a.host, _, err = net.SplitHostPort(addr)
	if err != nil {
		log.Fatalf("%s: addr must be host:port", pos(tree, "addr"))
	}
	a.a = smtp.PlainAuth("", a.username, password, a.host)
	a.c, err = smtp.Dial(addr)
	if err != nil {
		log.Fatal(err)
	}

	v := tree.Get("destinations")
	if v == nil {
		log.Fatalf("%s: no %q table", pos(tree, ""), "destinations")
	}
	trees, ok := v.([]*toml.TomlTree)
	if !ok {
		log.Fatalf("%s: %q is not a table", pos(tree, "destinations"), "destinations")
	}

	a.msg = []byte("From: ")
	if name == "" {
		a.msg = append(a.msg, a.username...)
	} else {
		a.msg = []byte(fmt.Sprintf("%s <%s>", name, a.username))
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
			a.msg = append(a.msg, fmt.Sprintf("%s <%s>", name, email)...)
		}
		if i != len(trees)-1 {
			a.msg = append(a.msg, ',')
		}
		a.msg = append(a.msg, "\r\n"...)
	}
	a.msg = append(a.msg, "Subject: "...)
	a.mlen = len(a.msg)

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

func pos(tree *toml.TomlTree, key string) string {
	p := tree.GetPosition(key)
	return fmt.Sprintf("pos %dl %dc", p.Line, p.Col)
}

func main() {
	path := flag.String("c", "/usr/local/etc/systemd-monitor/config.json", "path to configuration file")
	flag.Parse()
	tree, err := toml.LoadFile(*path)
	if err != nil {
		log.Fatal(err)
	}

	v := tree.Get("accounts")
	if v == nil {
		log.Fatalf("%s: no %q, array of tables", pos(tree, ""), "accounts")
	}
	trees, ok := v.([]*toml.TomlTree)
	if !ok {
		log.Fatalf("%s: %q is not an array of tables", pos(tree, "accounts"), "accounts")
	}

	accounts := make([]*account, len(trees))
	var i int
	for i, tree = range trees {
		accounts[i] = new(account)
		accounts[i].init(tree)
	}
	log.Printf("%q", accounts[0].msg)
	return

	s := journal()
	log.Print("initialized")

	scanLoop(accounts, s)
}

func scanLoop(accounts []*account, s *bufio.Scanner) {
	var wg sync.WaitGroup
	for s.Scan() {
		l := s.Text()
		i := strings.Index(l, "]: ")
		if i == -1 {
			log.Printf("line does not contain \"]: \": %q", l)
			continue
		}
		i += 3
		j := strings.Index(l, ": U")
		if j == -1 {
			log.Printf("line does not contain \": U\": %q", l)
			continue
		}
		unit := l[i:j]
		log.Printf("%s failed", unit)
		out, err := exec.Command("systemctl", "--full", "status", unit).Output()
		if err != nil && out == nil {
			log.Print(err)
		}
		subject := fmt.Sprintf("%s failed", unit)
		wg.Add(len(accounts))
		for _, a := range accounts {
			go func(a *account) {
				a.send(subject, out)
				wg.Done()
			}(a)
		}
		wg.Wait()
	}
	if err := s.Err(); err != nil {
		log.Fatal(err)
	}
}

func journal() *bufio.Scanner {
	cmd := exec.Command("journalctl", "-f", "-b", "-q", "--no-tail", "CODE_FUNCTION=unit_notify")
	w, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	return bufio.NewScanner(w)
}

func (a *account) send(subject string, body []byte) {
	log.Printf("%s: sending emails", a.username)
	if err := a.sendMail(subject, body); err != nil {
		log.Printf("%s: error: %s", a.username, err)
		if a.backup != nil {
			log.Printf("%s: falling back to: %s", a.backup.username)
			a.backup.send(subject, body)
		}
		return
	}
	log.Printf("%s: sent emails to %s", a.username)
}

func (a *account) sendMail(subject string, body []byte) error {
	a.msg = append(a.msg, subject...)
	a.msg = append(a.msg, body...)
	var ok bool
	var err error
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
	if err = a.c.Mail(a.username); err != nil {
		return err
	}
	for _, addr := range a.destinations {
		if err = a.c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := a.c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(a.msg)
	a.msg = a.msg[:a.mlen]
	return err
}
