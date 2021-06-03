package cmd

import (
	"fmt"
	"strings"

	"github.com/urfave/cli"

	"github.com/longhorn/backing-image-file-provisioner/pkg/server"
)

func Start(c *cli.Context) error {
	listen := c.String("listen")
	fileName := c.String("file-name")
	sourceType := c.String("source-type")
	parameters, err := parseSliceToMap(c.StringSlice("parameters"))
	if err != nil {
		return err
	}

	return server.NewServer(listen, fileName, sourceType, parameters)
}

func parseSliceToMap(sli []string) (map[string]string, error) {
	res := map[string]string{}
	for _, s := range sli {
		kvPair := strings.Split(s, "=")
		if len(kvPair) != 2 {
			return nil, fmt.Errorf("invalid slice input %v since it cannot be converted to a map entry", kvPair)
		}
		if kvPair[0] == "" {
			return nil, fmt.Errorf("invalid slice input %v due to the empty key", kvPair)
		}
		res[kvPair[0]] = kvPair[1]
	}
	return res, nil
}
