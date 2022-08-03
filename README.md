# google-ddns-updater

This application updates [Google Domain](https://domains.google.com)'s DDNS configurations.

It caches values and skip requests if there were no changes, so that it is not blocked by Google.

## how to configure

```bash
$ cp config.json.sample config.json
$ vi config.json
```

![screenshot_google_ddns_updater](https://user-images.githubusercontent.com/185988/58552758-20bfa000-824e-11e9-9d11-13e29bd0bef6.jpg)

Put your usernames and passwords in it.

### ip address

If you want to update your DDNS configurations with a certain ip address, put `ip` in your config file like:

```json
{
  "ip": "1.2.3.4",
  "configs": [
    {
      "hostname": "ddns.some-domain.com",
      "username": "no-such-username",
      "password": "wrong-password"
    }
  ]
}
```

Otherwise, it will fetch your external ip address from [domains.google.com/checkip](https://domains.google.com/checkip) automatically.

## how to build/install

```bash
$ git clone https://github.com/meinside/google-ddns-updater.git
$ cd google-ddns-updater/
$ go build
```

or

```bash
$ go install github.com/meinside/google-ddns-updater@latest
```

## how to run

Run it with a config file at certain location:

```bash
$ ./google-ddns-updater -c /etc/ddns/config1.json
# or
$ ./google-ddns-updater --config /etc/ddns/config2.json
```

Or run with a config file which is located in the same directory as the binary:

```bash
$ /path/to/google-ddns-updater
# will load configs from /path/to/config.json
```

If you want to update specific domains only:

```bash
$ /path/to/google-ddns-updater some.domain.com another.domain.com andanother.domain.com
```

When you want to set your ip address manually, use `-i` or `--ip` argument:

```bash
$ /path/to/google-ddns-updater -i 123.123.123.123
```

### priority of ip address

**command line argument** > **configs file** > **external ip** (from [domains.google.com/checkip](https://domains.google.com/checkip))

## crontab

Following is a sample crontab:

```
0 6 * * * /path/to/google-ddns-updater -c /where/is/config1.json
0 7 * * * /path/to/google-ddns-updater -c /where/is/config2.json some.domain.com
0 8 * * * /path/to/google-ddns-updater -c /where/is/config3.json another.domain.com andanother.domain.com
0 9 * * * /path/to/google-ddns-updater -c /where/is/config4.json -i 12.23.34.45
```

## license

MIT

