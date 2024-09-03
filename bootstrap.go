package main

import (
	"log"
	"os"

	"github.com/cookieai-jar/go-scim/cmd/api"
	"github.com/cookieai-jar/go-scim/cmd/groupsync"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "scim",
		Usage: "Simple Cloud Identity Management",
		Commands: []*cli.Command{
			api.Command(),
			groupsync.Command(),
		},
		HideVersion: true,
		Authors: []*cli.Author{
			{
				Name:  "Weinan Qiu",
				Email: "davidiamyou@gmail.com",
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
