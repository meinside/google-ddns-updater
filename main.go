package main

// google-ddns-updater
//
// Update Google Domain's DDNS configurations
//
// https://support.google.com/domains/answer/6147083?hl=en
//
// created on:  2019.03.04.
// last update: 2020.10.16.
//
//
// cronjob example:
//
//   0 6 * * * /path/to/google-ddns-updater -c /where/is/config.json
//   0 7 * * * /path/to/google-ddns-updater -c /where/is/config.json some.domain.com
//   0 8 * * * /path/to/google-ddns-updater -c /where/is/config.json another.domain.com andanother.domain.com

import (
	"log"
	"os"
	"path/filepath"

	"github.com/meinside/google-ddns-updater/helper"
)

func main() {
	var confs helper.Configs
	var err error

	// command line arguments
	args := os.Args[1:]

	// read params from arguments
	var needIP, needConf bool
	var ipAddr string
	hostnames := []string{}
	confFilepath := helper.DefaultConfFilepath()
	for _, arg := range args {
		if arg == "-h" || arg == "--help" { // help flag
			helper.ExitWithHelpMessage()
		} else if arg == "-i" || arg == "--ip" { // ip flag
			if needConf { // wrong param was given
				helper.ExitWithHelpMessage()
			}

			needIP = true
		} else if arg == "-c" || arg == "--config" { // configs flag
			if needIP { // wrong param was given
				helper.ExitWithHelpMessage()
			}

			needConf = true
		} else if needIP {
			ipAddr = arg

			needIP = false
		} else if needConf { // configs filepath
			confFilepath = arg

			needConf = false
		} else { // hostnames
			hostnames = append(hostnames, arg)
		}
	}
	if needIP || needConf { // needed params were not given
		helper.ExitWithHelpMessage()
	}

	// load configs
	if confs, err = helper.ReadConfigs(confFilepath); err == nil {
		log.Printf("loaded configs file at: %s", confFilepath)
	} else {
		helper.ExitWithError("failed to read configs file at: %s", confFilepath)
	}

	// if no hosts were given,
	if len(hostnames) <= 0 {
		// load all hosts from configs
		for _, conf := range confs.Configs {
			hostnames = append(hostnames, conf.Hostname)
		}
	}

	// if no ip address was given, load it from the configs
	if ipAddr == "" {
		ipAddr = confs.IPAddress
	}

	// if ip address was not in the configs, fetch it from google domains
	if ipAddr == "" {
		if ipAddr, err = helper.GetExternalIP(); err == nil {
			log.Printf("fetched external ip: %s", ipAddr)
		}
	}

	// will not work without an ip address...
	if ipAddr != "" {
		cacheDir := filepath.Dir(confFilepath)

		for _, hostname := range hostnames {
			log.Printf("processing hostname: %s", hostname)

			conf := helper.ConfigForHostname(confs, hostname)
			if conf == nil {
				log.Printf("no such hostname: %s in the configs", hostname)
				continue
			}

			// read cached ip address,
			var savedIP string
			if savedIP, err = helper.LoadCachedIP(*conf, cacheDir); err == nil {
				if ipAddr != savedIP {
					if updateErr := helper.UpdateIP(*conf, cacheDir, ipAddr); updateErr != nil {
						err = updateErr

						log.Printf("failed to update ip: %s for hostname: %s (%s)", ipAddr, conf.Hostname, err)
					}
				} else {
					log.Printf("cached ip address: %s is already set for hostname: %s", savedIP, conf.Hostname)
				}
			}
		}
	}

	// check error
	if err != nil {
		helper.ExitWithError(err.Error())
	}

	log.Printf("update finished")
}
