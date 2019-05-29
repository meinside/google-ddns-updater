package main

// google-ddns-updater
//
// Update Google Domain's DDNS configurations
//
// https://support.google.com/domains/answer/6147083?hl=en
//
// created on:  2019.03.04.
// last update: 2019.05.29.
//
//
// cronjob example:
//
//   0 6 * * * /path/to/google-ddns-updater -c /where/is/config.json
//   0 7 * * * /path/to/google-ddns-updater -c /where/is/config.json some.domain.com
//   0 8 * * * /path/to/google-ddns-updater -c /where/is/config.json another.domain.com andanother.domain.com

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// config struct
//
// example:
//
//{
//	"configs": [
//		{
//			"hostname": "YOUR-SUBDOMAIN1.DOMAIN.TLD",
//			"username": "0123456789abcdefg",
//			"password": "abcdefg0123456789"
//		},
//		{
//			"hostname": "YOUR-SUBDOMAIN2.DOMAIN.TLD",
//			"username": "9876543210abcdefg",
//			"password": "abcdefg9876543210"
//		}
//	]
//}
type configs struct {
	Configs []config `json:"configs"`
}

type config struct {
	Hostname string `json:"hostname"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// constants
const (
	defaultConfigFilename = "config.json"
	ipCacheFilename       = "ip.cache"

	checkIPURL   = "https://domains.google.com/checkip"
	apiURLFormat = "https://%s:%s@domains.google.com/nic/update?hostname=%s&myip=%s"
	userAgent    = "Google-DDNS-Updater/Golang"
	fallbackIP   = "0.0.0.0"
)

// get current directory
func pwd() string {
	var err error
	var execFilepath string
	if execFilepath, err = os.Executable(); err == nil {
		return filepath.Dir(execFilepath)
	}

	panic(err)
}

func defaultConfFilepath() string {
	return filepath.Join(pwd(), defaultConfigFilename)
}

// read configs file
func readConfigs(filepath string) (result configs, err error) {
	var file []byte
	file, err = ioutil.ReadFile(filepath)
	if err == nil {
		if err = json.Unmarshal(file, &result); err == nil {
			return result, nil
		}
	}

	return configs{}, err
}

// get external IP address (https://gist.github.com/jniltinho/9788121)
func getExternalIP() (string, error) {
	var err error

	httpClient := defaultHTTPClient()

	// http get request
	var req *http.Request
	if req, err = http.NewRequest("GET", checkIPURL, nil); err == nil {
		// user-agent
		req.Header.Set("User-Agent", userAgent)

		// http get
		var resp *http.Response
		resp, err = httpClient.Do(req)

		if resp != nil {
			defer resp.Body.Close() // in case of http redirects
		}

		if err == nil && resp.StatusCode == 200 {
			var body []byte
			if body, err = ioutil.ReadAll(resp.Body); err == nil {
				ip := strings.TrimSpace(string(body))

				return ip, nil
			}

			err = fmt.Errorf("failed to read external ip: %s", err)
		} else {
			err = fmt.Errorf("failed to fetch external ip: %s (http %d)", err, resp.StatusCode)
		}
	}

	return fallbackIP, err
}

// get ip cache file's path
func ipCacheFilepath(hostname string) string {
	return filepath.Join(pwd(), ipCacheFilename+"."+hostname)
}

// load cached ip for given config
func loadCachedIP(conf config) (string, error) {
	var err error

	filepath := ipCacheFilepath(conf.Hostname)

	if _, err = os.Stat(filepath); err != nil && os.IsNotExist(err) {
		log.Printf("ip cache file: %s does not exist", filepath)

		cacheIP(conf, fallbackIP)

		return fallbackIP, nil
	}

	var data []byte
	data, err = ioutil.ReadFile(filepath)

	if err == nil {
		log.Printf("loaded cached ip: %s from file: %s", string(data), filepath)
	}

	return string(data), err
}

// cache ip locally
func cacheIP(conf config, ip string) error {
	filepath := ipCacheFilepath(conf.Hostname)

	log.Printf("caching ip: %s to file: %s", ip, filepath)

	return ioutil.WriteFile(filepath, []byte(ip), 0644)
}

// update ip
func updateIP(conf config, ip string) error {
	var err error

	httpClient := defaultHTTPClient()

	// api url
	apiURL := fmt.Sprintf(apiURLFormat, conf.Username, conf.Password, conf.Hostname, ip)

	// http post request
	var req *http.Request
	if req, err = http.NewRequest("POST", apiURL, nil); err == nil {
		// user-agent
		req.Header.Set("User-Agent", userAgent)

		// http post
		var resp *http.Response
		resp, err = httpClient.Do(req)

		if resp != nil {
			defer resp.Body.Close()
		}

		if err == nil {
			var bytes []byte
			if bytes, err = ioutil.ReadAll(resp.Body); err == nil {
				err = checkResponse(conf, string(bytes), ip)
			}
		}
	}

	return err
}

// check response from google domains
func checkResponse(conf config, res, ip string) error {
	var err error

	//log.Printf("response from google domains: %s", res)

	comps := strings.Split(res, " ")

	if len(comps) >= 2 {
		// success
		if ip == comps[1] {
			cacheIP(conf, ip)
		} else {
			err = fmt.Errorf("returned ip differs from the requested one: %s and %s", comps[1], ip)
		}
		switch comps[0] {
		case "good":
			log.Printf("update was successful")
		case "nochg":
			log.Printf("ip address: %s is already set for hostname: %s", ip, conf.Hostname)
		}
	} else {
		// errors
		switch res {
		case "nohost":
			err = fmt.Errorf("hostname: %s does not exist, or does not have ddns enabled", conf.Hostname)
		case "badauth":
			err = fmt.Errorf("username and password combination: %s / %s is not valid for hostname: %s", conf.Username, conf.Password, conf.Hostname)
		case "notfqdn":
			err = fmt.Errorf("supplied hostname: %s is not a valid fully-qualified domain name", conf.Hostname)
		case "badagent":
			err = fmt.Errorf("user agent: %s is not valid", userAgent)
		case "abuse":
			err = fmt.Errorf("access for the hostname: %s has been blocked due to failure to interpret previous responses correctly", conf.Hostname)
		case "911":
			err = fmt.Errorf("internal server error on google")
		default:
			err = fmt.Errorf("unhandled response from server: %s", res)
		}
	}

	return err
}

// get default http client
func defaultHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func configForHostname(confs configs, hostname string) *config {
	for _, conf := range confs.Configs {
		if conf.Hostname == hostname {
			return &conf
		}
	}

	return nil
}

// exit program with error message
func exitWithError(format string, a ...interface{}) {
	log.Printf(format, a...)

	os.Exit(1)
}

func main() {
	var confs configs
	var err error

	// command line arguments
	args := os.Args[1:]

	// read params and configs' filepath from args
	hostnames := []string{}
	isConf := false
	confFilepath := defaultConfFilepath()
	for _, arg := range args {
		if arg == "-c" || arg == "--config" { // configs flag
			isConf = true
		} else if isConf { // configs filepath
			confFilepath = arg
			isConf = false
		} else { // hostnames
			hostnames = append(hostnames, arg)
			isConf = false
		}
	}

	// load configs
	if confs, err = readConfigs(confFilepath); err == nil {
		log.Printf("loaded configs file at: %s", confFilepath)
	} else {
		exitWithError("failed to read configs file at: %s", confFilepath)
	}

	// if no hosts were given,
	if len(hostnames) <= 0 {
		// load all hosts from configs
		for _, conf := range confs.Configs {
			hostnames = append(hostnames, conf.Hostname)
		}
	}

	// fetch external ip address,
	var currentIP string
	if currentIP, err = getExternalIP(); err == nil {
		log.Printf("fetched external ip: %s", currentIP)

		for _, hostname := range hostnames {
			log.Printf("processing hostname: %s", hostname)

			conf := configForHostname(confs, hostname)
			if conf == nil {
				log.Printf("no such hostname: %s in the configs", hostname)
				continue
			}

			// read cached ip address,
			var savedIP string
			if savedIP, err = loadCachedIP(*conf); err == nil {
				if currentIP != savedIP {
					if updateErr := updateIP(*conf, currentIP); updateErr != nil {
						err = updateErr

						log.Printf("failed to update ip: %s for hostname: %s (%s)", currentIP, conf.Hostname, err)
					}
				} else {
					log.Printf("cached ip address: %s is already set for hostname: %s", savedIP, conf.Hostname)
				}
			}
		}
	} else {
		log.Printf("failed to fetch external ip: %s", err)
	}

	// check error
	if err != nil {
		exitWithError(err.Error())
	}

	log.Printf("update finished")
}
