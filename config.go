package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/nhooyr/color/log"

	"gopkg.in/gomail.v2"
)

type config struct {
	Name   string   `json:"name"`
	From   string   `json:"from"`
	Emails []*email `json:"emails"`
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
	from := (&gomail.Message{}).FormatAddress(c.From, c.Name)
	for _, e := range c.Emails {
		e.init(from)
	}
}
