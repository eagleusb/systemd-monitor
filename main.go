package main

import (
	"bytes"
	"flag"
	"os/exec"
	"sync"
	"time"

	"github.com/coreos/go-systemd/dbus"
	"github.com/nhooyr/color/log"
	"github.com/pelletier/go-toml"
)

var accounts []*account

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

	accounts = make([]*account, len(trees))
	var i int
	for i, tree = range trees {
		accounts[i] = new(account)
		accounts[i].init(tree)
	}
	log.Print("initialized")

	monitor()
}

var lf = []byte{'\n'}
var crlf = []byte{'\r', '\n'}

func monitor() {
	conn, err := dbus.New()
	if err != nil {
		broadcast("error connecting to systemd dbus", []byte(err.Error()))
		log.Fatal(err)
	}
	err = conn.Subscribe()
	if err != nil {
		broadcast("error subscribing to systemd dbus", []byte(err.Error()))
		log.Fatal(err)
	}
	log.Print("subscribed to systemd dbus")
	eventChan, errChan := conn.SubscribeUnits(time.Second)
	for {
		select {
		case events := <-eventChan:
			for service, event := range events {
				if event.ActiveState == "failed" {
					subject := service + " failed"
					log.Print(subject)
					out, err := exec.Command("systemctl", "--full", "status", service).Output()
					if err != nil && out == nil {
						broadcast("error getting unit status", []byte(err.Error()))
						log.Println("error getting unit status:", err)
					}
					out = bytes.Replace(out, lf, crlf, -1)
					broadcast(subject, out)
				}
			}
		case err := <-errChan:
			broadcast("error receiving event from systemd dbus", []byte(err.Error()))
			log.Fatal(err)
		}
	}
}

var wg sync.WaitGroup

func broadcast(subject string, body []byte) {
	wg.Add(len(accounts))
	for _, a := range accounts {
		go func(a *account) {
			a.send(subject, body)
			wg.Done()
		}(a)
	}
	wg.Wait()
}
