package dns

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/allegro/bigcache"
	"github.com/eko/gocache/lib/v4/marshaler"
	bcstore "github.com/eko/gocache/store/bigcache/v4"
	"github.com/flanksource/commons/logger"
	"github.com/pkg/errors"
)

type Cache struct {
	*marshaler.Marshaler
}

var cache *Cache

func NewCache() (*Cache, error) {
	bigcacheClient, _ := bigcache.NewBigCache(bigcache.DefaultConfig(60 * time.Minute))
	bigcacheStore := bcstore.NewBigcache(bigcacheClient)
	return &Cache{marshaler.New(bigcacheStore)}, nil
}

func init() {
	cache, _ = NewCache()
}

type IPs []net.IP

func CacheLookup(recordType, hostname string) ([]net.IP, error) {
	var ips IPs
	key := fmt.Sprintf("%s:%s", recordType, hostname)

	if _, err := cache.Get(context.TODO(), key, &ips); err == nil {
		return ips, nil
	}

	ips, err := Lookup(recordType, hostname)
	if err != nil {
		return nil, err
	}
	err = cache.Set(context.TODO(), key, ips, nil)
	return ips, err
}

func Lookup(recordType, hostname string) ([]net.IP, error) {
	host := hostname
	if url, err := url.Parse(hostname); err != nil {
		return nil, errors.Wrapf(err, "invalid IP/URL: %s", hostname)
	} else if url.Hostname() != "" {
		host = url.Hostname()
	}

	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, errors.Wrapf(err, "lookup of %s failed", host)
	}
	var ipv4 []net.IP
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4 = append(ipv4, ip)
		}
	}
	logger.Debugf("%s => %v", host, ipv4)
	return ipv4, nil
}
