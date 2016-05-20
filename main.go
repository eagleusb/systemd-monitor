package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/nhooyr/color/log"
	"github.com/satori/go.uuid"
	"gopkg.in/gomail.v2"
)

func main() {
	path := flag.String("c", "/usr/local/etc/systemd-monitor/config.json", "path to configuration file")
	flag.Parse()
	c := &config{queue: make(chan string), Workers: 2}
	c.readFile(*path)

	cmd := exec.Command("journalctl", "-f", "-b", "-q", "--no-tail", "CODE_FUNCTION=unit_notify")
	w, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < c.Workers; i++ {
		go c.worker()
	}
	log.Print("initialized")

	s := bufio.NewScanner(w)
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
		c.queue <- l[i:j]
	}
	if err = s.Err(); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	Emails  []*email `json:"emails"`
	Workers int      `json:"workers"`
	queue   chan string
}

func (c *config) readFile(path string) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	if err = json.Unmarshal(f, c); err != nil {
		log.Fatal(err)
	}
	if c.Workers < 1 {
		log.Fatal("must be atleast one worker")
	}
	for _, e := range c.Emails {
		e.init()
	}
}

var (
	user = os.Getenv("USER")
	from string
)

func init() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	from = user + "@" + hostname
}

func (c *config) worker() {
	for unit := range c.queue {
		id := uuid.NewV4().String()
		for _, e := range c.Emails {
			go e.send(id, unit)
		}
	}
}

type email struct {
	Name               string `json:name`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	Destination        string `json:"destination"`
	Backup             *email `json:"backup"`
	d                  *gomail.Dialer
}

func (e *email) init() {
	e.d = new(gomail.Dialer)
	if e.Destination == "" {
		log.Fatal("empty Destination: %+v", e)
	}
	if e.Host == "" {
		log.Fatal("empty Host: %+v", e)
	}
	e.d.Host = e.Host
	e.d.Port = e.Port
	e.d.Username = e.Username
	e.d.Password = e.Password
	if e.InsecureSkipVerify {
		e.d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if e.Backup != nil {
		e.Backup.init()
	}
}

// TODO id needs work
func (e *email) send(id string, unit string) {
	m := e.message(unit)
	log.Printf("%s: sending email to %s for %s", id, e.Destination, unit)
	if err := e.d.DialAndSend(m); err != nil {
		log.Printf("%s: error when sending to %s for %s: %s", id, e.Destination, unit, err)
		if e.Backup != nil {
			e.Backup.send(id, unit)
		}
	} else {
		log.Printf("%s: sent email to %s for %s", id, e.Destination, unit)
	}
}

func (e *email) message(unit string) *gomail.Message {
	return gomail.NewMessage(func(m *gomail.Message) {
		m.SetHeader("From", m.FormatAddress(from, user))
		m.SetHeader("To", m.FormatAddress(e.Destination, e.Name))
		m.SetHeader("Subject", unit+" failed")
		m.AddAlternativeWriter("text/plain; charset=UTF-8", func(w io.Writer) error {
			cmd := exec.Command("systemctl", "--full", "status", unit)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				log.Print(err)
				return nil
			}
			err = cmd.Start()
			if err != nil {
				log.Print(err)
				return nil
			}
			_, err = io.Copy(w, stdout)
			if err != nil {
				log.Print(err)
			}
			return nil
		})
	})
}
