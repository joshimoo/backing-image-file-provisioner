package main

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/longhorn/backing-image-file-provisioner/app/cmd"
	"github.com/longhorn/backing-image-file-provisioner/pkg/types"
)

func main() {
	a := cli.NewApp()

	a.Before = func(c *cli.Context) error {
		if c.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	a.Flags = []cli.Flag{
		cli.BoolFlag{
			Name: "debug",
		},
		cli.StringFlag{
			Name:  "listen",
			Value: ":" + strconv.Itoa(types.DefaultPort),
		},
		cli.StringFlag{
			Name: "type",
		},
		cli.StringSliceFlag{
			Name: "parameters",
		},
	}
	a.Action = func(c *cli.Context) {
		if err := cmd.Start(c); err != nil {
			logrus.Fatalf("Error running file provisioner command: %v.", err)
		}
	}
	if err := a.Run(os.Args); err != nil {
		logrus.Fatal("Error when executing command: ", err)
	}
}
