package config

import (
	"errors"
	"flag"
	"net"
	"regexp"
	"strconv"
	"time"
)

type Config struct {
	Addr        string
	DBUrl       string
	TokenSecret string
	TokenTTL    time.Duration
	Debug       bool
}

func ParseFlags() (cfg Config, err error) {
	var host string
	flag.StringVar(&host, "host", "0.0.0.0", "listen host name (default 0.0.0.0)")
	var port uint
	flag.UintVar(&port, "port", 80, "listen port number (default 80)")
	flag.StringVar(&cfg.DBUrl, "db-url", "qsurvey.sqlite", "path to SQLite3 DB file (default qsurvey.sqlite)")
	flag.StringVar(&cfg.TokenSecret, "token-secret", "", "secret key for token encryption and decryption")
	var ttl uint
	flag.UintVar(&ttl, "token-ttl", 120, "token TTL in seconds (default 120)")
	flag.BoolVar(&cfg.Debug, "debug", false, "log at DEBUG level")
	flag.Parse()

	cfg.Addr = net.JoinHostPort(host, strconv.Itoa(int(port)))
	cfg.TokenTTL = time.Duration(ttl) * time.Second

	if cfg.TokenSecret == "" {
		err = errors.New("missing parameter -token-secret")
	}

	return
}

func (cfg Config) Url() (url string) {
	url = cfg.Addr
	url = regexp.MustCompile(`^0.0.0.0`).ReplaceAllString(url, "localhost")
	url = "http://" + url
	return
}
