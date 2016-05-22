package main

import (
	"bufio"
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
	c            *smtp.Client
	a            smtp.Auth
	msg          []byte
	destinations []*destination
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

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		log.Fatalf("%s: addr must be host:port", pos(tree, "addr"))
	}
	a.a = smtp.PlainAuth("", a.username, password, host)
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

	var from string
	if name == "" {
		from = a.username
	} else {
		from = fmt.Sprintf("%s <%s>", name, a.username)
	}
	for _, tree := range trees {
		name := optional(tree, "name")
		email := necessary(tree, "email")
		var to string
		if name == "" {
			to = email
		} else {
			to = fmt.Sprintf("%s <%s>", name, email)
		}
		a.destinations = append(a.destinations, &destination{email,
			[]byte(fmt.Sprintf("From: %s\r\n", from) +
				fmt.Sprintf("To: %s\r\n", to) +
				"Subject: ")})
	}

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

type destination struct {
	email string
	msg   []byte
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
			for _, d := range a.destinations {
				go func(a *account, d *destination) {
					log.Printf("%s: sending email to %s", a.username, d.email)
					a.send(subject, out)
					wg.Done()
				}(a, d)
			}
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
	for _, d := range a.destinations
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

func (a *account) sendMail(subject string, body []byte) error {
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
}
