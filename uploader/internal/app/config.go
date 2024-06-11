package app

import (
	"fmt"
	"time"

	"golang.org/x/xerrors"
)

const (
	defaultHTTPHandlerTimeout = 2 * time.Minute
	defaultMaxExcelFileSize   = 1024 * 1024 * 10

	defaultAuthCookieName = "Session_id"
)

// Config is an app config.
type Config struct {
	HTTPAddr           string        `yaml:"http_addr"`
	DebugHTTPAddr      string        `yaml:"debug_http_addr"`
	HTTPHandlerTimeout time.Duration `yaml:"http_handler_timeout"`
	MaxExcelFileSize   int           `yaml:"max_excel_file_size_bytes"`
	// AuthCookieName is a request cookie that service forwards to YT.
	// YT proxy uses this cookie to authorize requester.
	// Session_id by default.
	AuthCookieName string `yaml:"auth_cookie_name"`

	Clusters        []*ClusterConfig          `yaml:"clusters"`
	clustersByProxy map[string]*ClusterConfig `yaml:"-"`
}

func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	type plain Config
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	if c.HTTPAddr == "" {
		return xerrors.New("http addr can not be empty")
	}
	if c.DebugHTTPAddr == "" {
		return xerrors.New("debug http addr can not be empty")
	}

	if c.HTTPHandlerTimeout == 0 {
		c.HTTPHandlerTimeout = defaultHTTPHandlerTimeout
	}

	if c.MaxExcelFileSize == 0 {
		c.MaxExcelFileSize = defaultMaxExcelFileSize
	}

	if c.AuthCookieName == "" {
		c.AuthCookieName = defaultAuthCookieName
	}

	if len(c.Clusters) == 0 {
		return xerrors.New("clusters can not be empty")
	}

	byProxy := make(map[string]*ClusterConfig)
	for _, conf := range c.Clusters {
		if _, ok := byProxy[conf.Proxy]; ok {
			return fmt.Errorf("duplicate cluster %s", conf.Proxy)
		}
		byProxy[conf.Proxy] = conf
		if conf.APIEndpointName == "" {
			conf.APIEndpointName = conf.Proxy
		}
	}
	c.clustersByProxy = byProxy

	return nil
}

type ClusterConfig struct {
	// Proxy identifies cluster.
	Proxy string `yaml:"proxy"`
	// APIEndpointName is an optional http api endpoint name.
	//
	// Equals to Proxy by default.
	APIEndpointName string `yaml:"api_endpoint_name"`
}
