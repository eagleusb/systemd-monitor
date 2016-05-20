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

	cmd := exec.Command("journalctl", "-f", "-b", "-q", "--no-tail")
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
		if l := s.Text(); strings.HasSuffix(l, ": Unit entered failed state.") {
			i := strings.Index(l, "]: ")
			if i == -1 {
				log.Printf("line does not contain \"]: \": %q", l)
				continue
			}
			i += 3
			j := strings.Index(l, ": U")
			c.queue <- l[i:j]
		}
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

type email struct {
	Name               string `json:name`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	Destination        string `json:"destination"`
	d                  *gomail.Dialer
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
		e.d = &gomail.Dialer{TLSConfig: new(tls.Config)}
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
		e.d.TLSConfig.InsecureSkipVerify = e.InsecureSkipVerify
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
		for _, e := range c.Emails {
			id := uuid.NewV4()
			m := newMessage(unit, e)
			log.Printf("%s: sending email to %s for %s", id, e.Destination, unit)
			if err := e.d.DialAndSend(m); err != nil {
				log.Printf("%s: %s", id, err)
				continue
			}
			log.Printf("%s: completed", id)
			break
		}
	}
}

func newMessage(unit string, e *email) *gomail.Message {
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
