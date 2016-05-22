package main

import (
	"bufio"
	"flag"
	"fmt"
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
