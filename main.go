package main

import (
	"bufio"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/nhooyr/color/log"
)

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
		if err != nil {
			log.Print(err)
		}
		body := string(out)
		subject := fmt.Sprintf("%s failed", unit)
		wg.Add(len(c.Emails))
		for _, e := range c.Emails {
			go func(e *email) {
				log.Printf("sending email to %s", e.Destination)
				e.send(subject, body)
				wg.Done()
			}(e)
		}
		wg.Wait()
	}
	if err = s.Err(); err != nil {
		log.Fatal(err)
	}
}
