package helper

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Configs struct for configurations
//
// config.json (example)
//
//	{
//	 "ip": "999.999.999.999",
//	 "Configs": [
//	   {
//	     "hostname": "YOUR-SUBDOMAIN1.DOMAIN.TLD",
//	     "username": "0123456789abcdefg",
//	     "password": "abcdefg0123456789"
//	   },
//	   {
//	     "hostname": "YOUR-SUBDOMAIN2.DOMAIN.TLD",
//	     "username": "9876543210abcdefg",
//	     "password": "abcdefg9876543210"
//	   }
//	 ]
//	}
type Configs struct {
	IPAddress string   `json:"ip,omitempty"`
	Configs   []Config `json:"configs"`
}

// Config struct for each configuration
type Config struct {
	Hostname string `json:"hostname"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// constants
const (
	version = "0.0.5" // bump this!

	defaultConfigFilename = "config.json"
	ipCacheFilename       = "ip.cache"

	checkIPURL      = "https://domains.google.com/checkip"
	apiURLFormat    = "https://%s:%s@domains.google.com/nic/update?hostname=%s&myip=%s"
	userAgentFormat = "Google-DDNS-Updater/%s (golang; %s; %s)"
	fallbackIP      = "0.0.0.0"
)

// user agent for this application
func userAgent() string {
	return fmt.Sprintf(userAgentFormat, version, runtime.GOOS, runtime.GOARCH)
}

// get current directory
func pwd() string {
	var err error
	var execFilepath string
	if execFilepath, err = os.Executable(); err == nil {
		return filepath.Dir(execFilepath)
	}

	panic(err)
}

// DefaultConfFilepath returns default config file's path
func DefaultConfFilepath() string {
	return filepath.Join(pwd(), defaultConfigFilename)
}

// ReadConfigs reads configs file
func ReadConfigs(filepath string) (result Configs, err error) {
	var file []byte
	file, err = os.ReadFile(filepath)
	if err == nil {
		if err = json.Unmarshal(file, &result); err == nil {
			return result, nil
		}
	}

	return Configs{}, err
}

// GetExternalIP gets external IP address of this host
func GetExternalIP() (string, error) {
	var err error

	httpClient := defaultHTTPClient()

	// http get request
	var req *http.Request
	if req, err = http.NewRequest("GET", checkIPURL, nil); err == nil {
		// user-agent
		req.Header.Set("User-Agent", userAgent())

		// http get
		var resp *http.Response
		resp, err = httpClient.Do(req)

		if resp != nil {
			defer resp.Body.Close() // in case of http redirects
		}

		if err == nil && resp.StatusCode == 200 {
			var body []byte
			if body, err = io.ReadAll(resp.Body); err == nil {
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
func ipCacheFilepath(cacheDir, hostname string) string {
	return filepath.Join(cacheDir, ipCacheFilename+"."+hostname)
}

// LoadCachedIP loads cached ip address for given config
func LoadCachedIP(conf Config, cacheDir string) (string, error) {
	var err error

	filepath := ipCacheFilepath(cacheDir, conf.Hostname)

	if _, err = os.Stat(filepath); err != nil && os.IsNotExist(err) {
		log.Printf("ip cache file: %s does not exist", filepath)

		_ = cacheIP(conf, cacheDir, fallbackIP)

		return fallbackIP, nil
	}

	var data []byte
	data, err = os.ReadFile(filepath)

	if err == nil {
		log.Printf("loaded cached ip: %s from file: %s", string(data), filepath)
	}

	return string(data), err
}

// cache ip locally
func cacheIP(conf Config, cacheDir, ip string) error {
	filepath := ipCacheFilepath(cacheDir, conf.Hostname)

	log.Printf("caching ip: %s to file: %s", ip, filepath)

	return os.WriteFile(filepath, []byte(ip), 0644)
}

// UpdateIP updates ip address for given config
func UpdateIP(conf Config, cacheDir, ip string) error {
	var err error

	httpClient := defaultHTTPClient()

	// api url
	apiURL := fmt.Sprintf(apiURLFormat, conf.Username, conf.Password, conf.Hostname, ip)

	// http post request
	var req *http.Request
	if req, err = http.NewRequest("POST", apiURL, nil); err == nil {
		// user-agent
		req.Header.Set("User-Agent", userAgent())

		// http post
		var resp *http.Response
		resp, err = httpClient.Do(req)

		if resp != nil {
			defer resp.Body.Close()
		}

		if err == nil {
			var bytes []byte
			if bytes, err = io.ReadAll(resp.Body); err == nil {
				err = checkResponse(conf, cacheDir, string(bytes), ip)
			}
		}
	}

	return err
}

// check response from google domains
func checkResponse(conf Config, cacheDir, response, ip string) error {
	var err error

	//log.Printf("response from google domains: %s", response)

	comps := strings.Split(response, " ")

	if len(comps) >= 2 {
		// success
		if ip == comps[1] {
			_ = cacheIP(conf, cacheDir, ip)
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
		switch response {
		case "nohost":
			err = fmt.Errorf("hostname: %s does not exist, or does not have ddns enabled", conf.Hostname)
		case "badauth":
			err = fmt.Errorf("username and password combination: %s / %s is not valid for hostname: %s", conf.Username, conf.Password, conf.Hostname)
		case "notfqdn":
			err = fmt.Errorf("supplied hostname: %s is not a valid fully-qualified domain name", conf.Hostname)
		case "badagent":
			err = fmt.Errorf("user agent: %s is not valid", userAgent())
		case "abuse":
			err = fmt.Errorf("access for the hostname: %s has been blocked due to failure to interpret previous responses correctly", conf.Hostname)
		case "911":
			err = fmt.Errorf("internal server error on google")
		default:
			err = fmt.Errorf("unhandled response from server: %s", response)
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

// ConfigForHostname returns a config for given hostname
func ConfigForHostname(confs Configs, hostname string) *Config {
	for _, conf := range confs.Configs {
		if conf.Hostname == hostname {
			return &conf
		}
	}

	return nil
}

// ExitWithError exits program with error message
func ExitWithError(format string, a ...interface{}) {
	log.Printf(format, a...)

	os.Exit(1)
}

// ExitWithHelpMessage exits program with help message
func ExitWithHelpMessage() {
	fmt.Printf(`
Help:

# show this help message
$ google-ddns-updater -h

# run updater with config file
$ google-ddns-updater -c /path/to/config-file.json

# update specific domains in config file
$ google-ddns-updater subdomain1.domain.com subdomain2.domain.com -c /path/to/config-file.json

# update specific domains with certain ip address
$ google-ddns-updater -i 255.255.255.255 subdomain1.domain.com subdomain2.domain.com -c /path/to/config-file.json
`)

	os.Exit(0)
}
