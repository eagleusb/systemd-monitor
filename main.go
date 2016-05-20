package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/nhooyr/color/log"
	"gopkg.in/gomail.v2"
)

var wg sync.WaitGroup

func main() {
	path := flag.String("c", "/usr/local/etc/systemd-monitor/config.json", "path to configuration file")
	flag.Parse()
	c := new(config)
	c.init(*path)

	cmd := exec.Command("journalctl", "-f", "-b", "-q", "--no-tail", "CODE_FUNCTION=unit_notify")
	w, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	s := bufio.NewScanner(w)
	log.Print("initialized")

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
		if err != nil {
			log.Print(err)
		}
		body := string(out)
		subject := fmt.Sprintf("%s failed", unit)
		var wg sync.WaitGroup
		wg.Add(len(c.Emails))
		for _, e := range c.Emails {
			log.Printf("sending email to %s", e.Destination)
			go e.send(subject, body)
		}
		wg.Wait()
	}
	if err = s.Err(); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	Emails []*email `json:"emails"`
	From   string   `json:"from"`
	Name   string   `json:"name"`
}

func (c *config) init(path string) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(f, c)
	if err != nil {
		log.Fatal(err)
	}
	if c.From == "" {
		var hostname string
		hostname, err = os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		user := os.Getenv("USER")
		c.From = user + "@" + hostname
		if c.Name == "" {
			c.Name = user
		}
	} else if c.Name == "" {
		c.Name = os.Getenv("USER")
	}
	from := (&gomail.Message{}).FormatAddress(c.From, c.Name)
	for _, e := range c.Emails {
		e.init(from)
	}
}

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
		log.Fatalf("empty Destination: %s", b)
	}
	if e.Host == "" {
		b, err := json.Marshal(e)
		if err != nil {
			log.Fatal(err)
		}
		log.Fatalf("empty Host: %s", b)
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
		log.Print(err)
		if e.Backup != nil {
			log.Printf("attempting to send to backup %s", e.Backup.Destination)
			e.Backup.send(subject, body)
			return
		}
	} else {
		log.Printf("sent email to %s", e.Destination)
	}
	wg.Done()
}
