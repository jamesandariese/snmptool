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

var community string
var mount string
var timeout time.Duration
var warning string
var critical string
var hostname string

var DriveNotFoundError = errors.New("drive not found")

func getSuffixForDrive(s *gosnmp.GoSNMP, mount string) (string, error) {
	c := make(chan gosnmp.SnmpPDU)
	go s.StreamWalk(".1.3.6.1.2.1.25.2.3.1.3", c)

	for s := range c {
		if s.Value == mount {
			return s.Name[strings.LastIndex(s.Name, ".")+1:], nil
		}
	}
	return "", DriveNotFoundError
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
				go s.StreamWalk(".1.3.6.1.2.1.25.2.3.1.3", ch)

				for s := range(ch) {
					fmt.Println(s.Value)
				}
				return nil
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
		cli.StringFlag{
			Name:        "C, community",
			Value:       "public",
			Usage:       "SNMP community string",
			Destination: &community,
		},
	}

	app.Action = func(ctx *cli.Context) error {
		if err := requireHostname(ctx); err != nil {
			return err
		}
		s, err := createSnmpClient()
		if err != nil {
			return err
		}
		suffix, err := getSuffixForDrive(s, mount)
		if err != nil {
			return cli.NewExitError("Couldn't find drive "+mount+": "+err.Error(), 3)
		}
		if suffix == "" {
			return cli.NewExitError("Couldn't find drive "+mount, 3)
		}

		size, err := s.Get(".1.3.6.1.2.1.25.2.3.1.5." + suffix)
		if err != nil {
			return cli.NewExitError("Error getting size of drive "+err.Error(), 3)
		}

		avail, err := s.Get(".1.3.6.1.2.1.25.2.3.1.6." + suffix)
		if err != nil {
			return cli.NewExitError("Error getting available space of drive "+err.Error(), 3)
		}

		freespace := float64(avail.Variables[0].Value.(int)) * 100 / (float64(size.Variables[0].Value.(int)))
		if criticalComparator, err := nagiosrangeparser.Compile(critical); err != nil {
			return cli.NewExitError(fmt.Sprintf("UNKNOWN: error parsing critical pattern %v: %#v", critical, err), 3)
		} else {
			if criticalComparator.Compare(freespace) {
				return cli.NewExitError(fmt.Sprintf("CRITICAL: %v %02.2f", mount, freespace), 2)
			}
		}
		if warningComparator, err := nagiosrangeparser.Compile(warning); err != nil {
			return cli.NewExitError(fmt.Sprintf("UNKNOWN: error parsing warning pattern %v: %#v", warning, err), 3)
		} else {
			if warningComparator.Compare(freespace) {
				return cli.NewExitError(fmt.Sprintf("WARNING: %v %02.2f", mount, freespace), 1)
			}
		}
		return cli.NewExitError(fmt.Sprintf("OK: %v %02.2f", mount, freespace), 0)

	}
	app.Run(os.Args)
}
