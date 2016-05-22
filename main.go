package main

import (
	"bufio"
	"flag"
	"os/exec"
	"strings"
	"sync"

	"github.com/nhooyr/color/log"
	"github.com/pelletier/go-toml"
)

func main() {
	path := flag.String("c", "/usr/local/etc/systemd-monitor/config.toml", "path to configuration file")
	flag.Parse()
	tree, err := toml.LoadFile(*path)
	if err != nil {
		log.Fatal(err)
	}

	v := tree.Get("accounts")
	if v == nil {
		log.Fatalf("%s: missing %q table of arrays", pos(tree, ""), "accounts")
	}
	trees, ok := v.([]*toml.TomlTree)
	if !ok {
		log.Fatalf("%s: type of %q is incorrect, should be array of tables", pos(tree, "accounts"), "accounts")
	}

	accounts := make([]*account, len(trees))
	var i int
	for i, tree = range trees {
		accounts[i] = new(account)
		accounts[i].init(tree)
	}

	s := journal()
	log.Print("tailing journal")

	tail(s, accounts)
}

func tail(s *bufio.Scanner, accounts []*account) {
	var wg sync.WaitGroup
	for s.Scan() {
		l := s.Text()
		i := strings.Index(l, "]: ")
		if i == -1 {
			log.Printf("error extracting unit name, journal line does not contain \"]: \": %q", l)
			continue
		}
		i += 3
		j := strings.Index(l, ": U")
		if j == -1 {
			log.Printf("error extracting unit name, journal line does not contain \": U\": %q", l)
			continue
		}
		unit := l[i:j]
		subject := unit + " failed"
		log.Print(subject)
		out, err := exec.Command("systemctl", "--full", "status", unit).Output()
		if err != nil && out == nil {
			log.Println("error getting unit status:", err)
		}
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
		log.Fatalln("fatal error", err)
	}
}

func journal() *bufio.Scanner {
	cmd := exec.Command("journalctl", "-f", "-b", "-q", "--no-tail", "CODE_FUNCTION=unit_notify")
	w, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalln("error tailing journal", err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatalln("error tailing journal", err)
	}
	return bufio.NewScanner(w)

}
