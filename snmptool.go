package main

import (
	"errors"
	"fmt"
	"github.com/alouca/gosnmp"
	"github.com/jamesandariese/nagiosrangeparser"
	"gopkg.in/urfave/cli.v1" // imports as package "cli"
	"os"
	"strings"
	"time"
)

var version string

var community string
var mount string
var timeout time.Duration
var warning string
var critical string
var hostname string

var OIDMappingNotFound = errors.New("OID mapping not found matching given string")

func getSuffixForString(s *gosnmp.GoSNMP, OID, mount string) (string, error) {
	c := make(chan gosnmp.SnmpPDU)
	go s.StreamWalk(OID, c)

	for s := range c {
		if s.Value == mount {
			return s.Name[strings.LastIndex(s.Name, ".")+1:], nil
		}
	}
	return "", OIDMappingNotFound
}

var NoValueFoundError = errors.New("no value found at associated OID")

func getAssociatedValue(s *gosnmp.GoSNMP, OID, associate string, remelts int, append string) (interface{}, error) {
	suffix, err := getSuffixForString(s, OID, associate)
	if err != nil {
		return "", err
	}

	for remelts > 0 {
		OID = OID[:strings.LastIndex(OID, ".")]
		remelts--
	}
	if append[0] != '.' {
		OID += "." + append
	} else {
		OID += append
	}
	OID += "." + suffix
	r, e := s.Get(OID)
	if e != nil {
		return nil, e
	}
	if len(r.Variables) < 1 {
		return nil, NoValueFoundError
	}
	return r.Variables[0].Value, nil
}

func createSnmpClient() (*gosnmp.GoSNMP, error) {
	s, err := gosnmp.NewGoSNMP(hostname, community, gosnmp.Version2c, int64(timeout.Seconds()))
	if err != nil {
		return nil, cli.NewExitError(fmt.Sprintf("Error connecting to %v: %#v", hostname, err), 3)
	}
	return s, nil
}

func requireHostname(c *cli.Context) error {
	if c.NArg() != 1 {
		return cli.NewExitError("A hostname is required", 3)
	}
	hostname = c.Args().First()
	return nil
}

func main() {
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Version = version
	app.Commands = []cli.Command{
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "list drives on host",
			Action: func(c *cli.Context) error {
				if err := requireHostname(c); err != nil {
					return err
				}
				s, err := createSnmpClient()
				if err != nil {
					return err
				}
				ch := make(chan gosnmp.SnmpPDU)
				go s.StreamWalk(".1.3.6.1.4.1.2021.9.1.2", ch)

				for s := range ch {
					fmt.Println(s.Value)
				}
				return nil
			},
		},
		{
			Name:  "disk",
			Usage: "check available disk space",
			Action: func(c *cli.Context) error {
				if err := requireHostname(c); err != nil {
					return err
				}
				s, err := createSnmpClient()
				if err != nil {
					return err
				}
				freespace, err := getAssociatedValue(s, ".1.3.6.1.4.1.2021.9.1.2", mount, 1, "9")
				if err != nil {
					return cli.NewExitError("Couldn't find drive free space:"+err.Error(), 3)
				}
				level, message, rc := nagiosrangeparser.NagiosComparator(warning, critical, float64(freespace.(int)))
				switch level {
				case "UNKNOWN":
					return cli.NewExitError(fmt.Sprintf("UNKNOWN: %s", message), rc)
				default:
					return cli.NewExitError(fmt.Sprintf("%s: %v %d%%", level, mount, freespace), rc)
				}
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "c, critical",
					Value:       "~:90",
					Usage:       "critical threshold",
					Destination: &critical,
				},
				cli.StringFlag{
					Name:        "w, warning",
					Value:       "~:80",
					Usage:       "warning threshold",
					Destination: &warning,
				},
				cli.StringFlag{
					Name:        "m, mount",
					Value:       "/",
					Usage:       "drive mount point to test",
					Destination: &mount,
				},
			},
		},
		{
			Name:  "inodes",
			Usage: "check available inodes",
			Action: func(c *cli.Context) error {
				if err := requireHostname(c); err != nil {
					return err
				}
				s, err := createSnmpClient()
				if err != nil {
					return err
				}
				freespace, err := getAssociatedValue(s, ".1.3.6.1.4.1.2021.9.1.2", mount, 1, "10")
				if err != nil {
					return cli.NewExitError("Couldn't find drive free space:"+err.Error(), 3)
				}
				level, message, rc := nagiosrangeparser.NagiosComparator(warning, critical, float64(freespace.(int)))
				switch level {
				case "UNKNOWN":
					return cli.NewExitError(fmt.Sprintf("UNKNOWN: %s", message), rc)
				default:
					return cli.NewExitError(fmt.Sprintf("%s: %v %d%%", level, mount, freespace), rc)
				}
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "c, critical",
					Value:       "~:90",
					Usage:       "critical threshold",
					Destination: &critical,
				},
				cli.StringFlag{
					Name:        "w, warning",
					Value:       "~:80",
					Usage:       "warning threshold",
					Destination: &warning,
				},
				cli.StringFlag{
					Name:        "m, mount",
					Value:       "/",
					Usage:       "drive mount point to test",
					Destination: &mount,
				},
			},
		},
	}

	app.Flags = []cli.Flag{
		cli.DurationFlag{
			Name:        "t, timeout",
			Value:       time.Second * 5,
			Usage:       "SNMP packet timeout",
			Destination: &timeout,
		},
		cli.StringFlag{
			Name:        "C, community",
			Value:       "public",
			Usage:       "SNMP community string",
			Destination: &community,
		},
	}
	app.Run(os.Args)
}
