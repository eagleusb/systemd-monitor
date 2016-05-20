package main

import (
	"encoding/json"
	"io/ioutil"
	"github.com/nhooyr/color/log"
	"os"

	"gopkg.in/gomail.v2"
)

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
