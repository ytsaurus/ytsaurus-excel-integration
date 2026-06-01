package app

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/xerrors"
)

// EventsConfig configures audit event logging.
type EventsConfig struct {
	// Enabled turns event logging on. False by default.
	Enabled bool `yaml:"enabled"`
	// LogPattern is a strftime pattern for rotated log files,
	// e.g. /var/log/excel-uploader/events.%Y%m%d%H%M.
	// Required when enabled is true.
	LogPattern string `yaml:"log_pattern"`
	// LinkName is the path to the symlink pointing to the current log file.
	LinkName string `yaml:"link_name"`
	// RotationTime controls how often to rotate. Defaults to 15 minutes.
	RotationTime time.Duration `yaml:"rotation_time"`
	// MaxAge is how long to keep old log files. Defaults to 7 days.
	MaxAge time.Duration `yaml:"max_age"`
}

const (
	defaultHTTPHandlerTimeout = 2 * time.Minute
	defaultMaxExcelFileSize   = 1024 * 1024 * 100

	defaultAuthCookieName = "Session_id"
	defaultSSOCookieName  = "yt_oauth_access_token"
)

// Config is an app config.
type Config struct {
	HTTPAddr           string        `yaml:"http_addr"`
	DebugHTTPAddr      string        `yaml:"debug_http_addr"`
	HTTPHandlerTimeout time.Duration `yaml:"http_handler_timeout"`
	MaxExcelFileSize   int           `yaml:"max_excel_file_size_bytes"`
	APIPathPrefix      string        `yaml:"api_path_prefix"`
	// AuthCookieName is a request cookie that service forwards to YT.
	// YT proxy uses this cookie to authorize requester.
	// Session_id by default.
	AuthCookieName string `yaml:"auth_cookie_name"`
	SSOCookieName  string `yaml:"sso_cookie_name"`

	CORS *CORSConfig `yaml:"cors"`

	Events *EventsConfig `yaml:"events"`

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

	if c.SSOCookieName == "" {
		c.SSOCookieName = defaultSSOCookieName
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

	c.APIPathPrefix = strings.Trim(c.APIPathPrefix, "/")

	return nil
}

type CORSConfig struct {
	// Allowed hosts is a list of allowed hostnames checked via exact match.
	AllowedHosts []string `yaml:"allowed_hosts"`
	// Allowed hosts is a list of allowed hostname suffixes checked via HasSuffix function.
	AllowedHostSuffixes []string `yaml:"allowed_host_suffixes"`
}

type ClusterConfig struct {
	// Proxy identifies cluster.
	Proxy string `yaml:"proxy"`
	// APIEndpointName is an optional http api endpoint name.
	//
	// Equals to Proxy by default.
	APIEndpointName string `yaml:"api_endpoint_name"`
	// UseTLS enables TLS for connections to this cluster.
	UseTLS bool `yaml:"use_tls"`
}
