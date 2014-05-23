package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/golang/glog"
)

const (
	SERVICE_DOMAINTYTPE = "service"
	URI_DOMAINTYPE      = "uri"
)

type Domain struct {
	typ    string
	value  string
}

type location struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func (s *location) equals(other *location) bool {
	if s == nil && other == nil {
		return true
	}

	return s != nil && other != nil &&
	s.Host == other.Host &&
	s.Port == other.Port
}

type Service struct {
	key      string
	location *location
	domain   string
	name     string
	status   *Status
}

type IoEtcdResolver struct {
	config          *Config
	watcher         *watcher
	domains         map[string]*Domain
	services    map[string]*ServiceCluster
	dest2ProxyCache map[string]http.Handler
	watchIndex      uint64
}

func NewEtcdResolver(c *Config) *IoEtcdResolver {
	domains := make(map[string]*Domain)
	services := make(map[string]*ServiceCluster)
	dest2ProxyCache := make(map[string]http.Handler)
	w := NewEtcdWatcher(c, domains, services)
	return &IoEtcdResolver{c, w, domains, services, dest2ProxyCache, 0}
}

func (r *IoEtcdResolver) init() {
	r.watcher.init()
}

func (domain *Domain) equals(other *Domain) bool {
	if domain == nil && other == nil {
		return true
	}

	return domain != nil && other != nil &&
		domain.typ == other.typ && domain.value == other.value
}

func (service *Service) equals(other *Service) bool {
	if(service == nil && other == nil) {
		return true
	}

	return service != nil && other != nil &&
		service.location.equals(other.location) &&
		service.status.equals(other.status)
}

func (r *IoEtcdResolver) resolve(domainName string) (http.Handler, error) {
	glog.V(5).Infof("Looking for domain : %s ", domainName)
	domain := r.domains[domainName]
	if domain != nil {
		switch domain.typ {

		case SERVICE_DOMAINTYTPE:
			if service, err := r.services[domain.value].Next(); err == nil {
				addr := net.JoinHostPort(service.location.Host, strconv.Itoa(service.location.Port))
				uri := fmt.Sprintf("http://%s/", addr)

				return r.getOrCreateProxyFor(uri), nil

			} else {
				return nil, err
			}
		case URI_DOMAINTYPE:
			return r.getOrCreateProxyFor(domain.value), nil
		}

	}
	glog.V(5).Infof("Domain %s not found", domainName)
	return nil, errors.New("Domain not found")
}

func (r *IoEtcdResolver) getOrCreateProxyFor(uri string) http.Handler {
	if _, ok := r.dest2ProxyCache[uri]; !ok {
		dest, _ := url.Parse(uri)
		r.dest2ProxyCache[uri] = httputil.NewSingleHostReverseProxy(dest)
	}
	return r.dest2ProxyCache[uri]
}
