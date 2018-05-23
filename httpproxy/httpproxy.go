package httpproxy

import (
	"net/http"
	"time"

	"github.com/phuslu/glog"

	"github.com/xuiv/goagent/httpproxy/filters"
	"github.com/xuiv/goagent/httpproxy/helpers"

	_ "github.com/xuiv/goagent/httpproxy/filters/auth"
	_ "github.com/xuiv/goagent/httpproxy/filters/autoproxy"
	_ "github.com/xuiv/goagent/httpproxy/filters/autorange"
	_ "github.com/xuiv/goagent/httpproxy/filters/direct"
	_ "github.com/xuiv/goagent/httpproxy/filters/gae"
	_ "github.com/xuiv/goagent/httpproxy/filters/php"
	_ "github.com/xuiv/goagent/httpproxy/filters/rewrite"
	_ "github.com/xuiv/goagent/httpproxy/filters/ssh2"
	_ "github.com/xuiv/goagent/httpproxy/filters/stripssl"
	_ "github.com/xuiv/goagent/httpproxy/filters/vps"
)

type Config struct {
	Enabled          bool
	Address          string
	KeepAlivePeriod  int
	ReadTimeout      int
	WriteTimeout     int
	RequestFilters   []string
	RoundTripFilters []string
	ResponseFilters  []string
}

func ServeProfile(config Config, branding string) error {

	listenOpts := &helpers.ListenOptions{TLSConfig: nil}

	ln, err := helpers.ListenTCP("tcp", config.Address, listenOpts)
	if err != nil {
		glog.Fatalf("ListenTCP(%s, %#v) error: %s", config.Address, listenOpts, err)
	}

	h := Handler{
		Listener:         ln,
		RequestFilters:   []filters.RequestFilter{},
		RoundTripFilters: []filters.RoundTripFilter{},
		ResponseFilters:  []filters.ResponseFilter{},
		Branding:         branding,
	}

	for _, name := range config.RequestFilters {
		f, err := filters.GetFilter(name)
		f1, ok := f.(filters.RequestFilter)
		if !ok {
			glog.Fatalf("%#v is not a RequestFilter, err=%+v", f, err)
		}
		h.RequestFilters = append(h.RequestFilters, f1)
	}

	for _, name := range config.RoundTripFilters {
		f, err := filters.GetFilter(name)
		f1, ok := f.(filters.RoundTripFilter)
		if !ok {
			glog.Fatalf("%#v is not a RoundTripFilter, err=%+v", f, err)
		}
		h.RoundTripFilters = append(h.RoundTripFilters, f1)
	}

	for _, name := range config.ResponseFilters {
		f, err := filters.GetFilter(name)
		f1, ok := f.(filters.ResponseFilter)
		if !ok {
			glog.Fatalf("%#v is not a ResponseFilter, err=%+v", f, err)
		}
		h.ResponseFilters = append(h.ResponseFilters, f1)
	}

	s := &http.Server{
		Handler:        h,
		ReadTimeout:    time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(config.WriteTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return s.Serve(h.Listener)
}
