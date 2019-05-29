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

## how to build

```bash
$ git clone https://github.com/meinside/google-ddns-updater.git
$ cd google-ddns-updater/
$ go build
```

or

```bash
$ go get -u github.com/meinside/google-ddns-updater
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

Following is a sample crontab:

```
0 6 * * * /path/to/google-ddns-updater -c /where/is/config.json
0 7 * * * /path/to/google-ddns-updater -c /where/is/config.json some.domain.com
0 8 * * * /path/to/google-ddns-updater -c /where/is/config.json another.domain.com andanother.domain.com
```

## license

MIT

