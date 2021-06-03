package cmd

import (
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/longhorn/backing-image-file-provisioner/pkg/client"
	"github.com/longhorn/backing-image-file-provisioner/pkg/types"
	"github.com/longhorn/backing-image-file-provisioner/pkg/util"
)

func FileCmd() cli.Command {
	return cli.Command{
		Name: "file",
		Subcommands: []cli.Command{
			GetCmd(),
			CloseCmd(),
		},
	}
}

func GetCmd() cli.Command {
	return cli.Command{
		Name: "get",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "url",
				Value: ":" + strconv.Itoa(types.DefaultPort),
			},
		},
		Action: func(c *cli.Context) {
			if err := get(c); err != nil {
				logrus.Fatalf("Error running get command: %v.", err)
			}
		},
	}
}

func get(c *cli.Context) error {
	cli := client.FileProvisionerClient{
		Remote: c.String("url"),
	}
	res, err := cli.Get()
	if err != nil {
		return err
	}
	return util.PrintJSON(res)
}

func CloseCmd() cli.Command {
	return cli.Command{
		Name: "close",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "url",
				Value: ":" + strconv.Itoa(types.DefaultPort),
			},
		},
		Action: func(c *cli.Context) {
			if err := serverClose(c); err != nil {
				logrus.Fatalf("Error running close command: %v.", err)
			}
		},
	}
}

func serverClose(c *cli.Context) error {
	cli := client.FileProvisionerClient{
		Remote: c.String("url"),
	}
	return cli.Close()
}
