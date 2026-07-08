package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App           string                 `yaml:"app" json:"app"`
	Domain        any                    `yaml:"domain" json:"domain"`
	Server        *ServerShorthand       `yaml:"server,omitempty" json:"server,omitempty"`
	Servers       map[string]ServerGroup `yaml:"servers,omitempty" json:"servers"`
	Build         Build                  `yaml:"build,omitempty" json:"build"`
	Deploy        Deploy                 `yaml:"deploy,omitempty" json:"deploy"`
	Services      map[string]Service     `yaml:"services,omitempty" json:"services,omitempty"`
	Accessories   map[string]Accessory   `yaml:"accessories,omitempty" json:"accessories,omitempty"`
	Secrets       Secrets                `yaml:"secrets,omitempty" json:"secrets"`
	Env           Env                    `yaml:"env,omitempty" json:"env,omitempty"`
	Proxy         Proxy                  `yaml:"proxy,omitempty" json:"proxy"`
	Registry      Registry               `yaml:"registry,omitempty" json:"registry"`
	Notifications Notifications          `yaml:"notifications,omitempty" json:"notifications"`
	Caddy         Caddy                  `yaml:"caddy,omitempty" json:"caddy,omitempty"`
	Dev           Dev                    `yaml:"dev,omitempty" json:"dev,omitempty"`
}

type ServerShorthand struct {
	Host string `yaml:"host" json:"host"`
	User string `yaml:"user" json:"user"`
	Port int    `yaml:"port,omitempty" json:"port,omitempty"`
	VPN  VPN    `yaml:"vpn,omitempty" json:"vpn,omitempty"`
}

type ServerGroup struct {
	Hosts []string `yaml:"hosts" json:"hosts"`
	User  string   `yaml:"user" json:"user"`
	Port  int      `yaml:"port,omitempty" json:"port,omitempty"`
	VPN   VPN      `yaml:"vpn,omitempty" json:"vpn,omitempty"`
}

type VPN struct {
	Required  bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Interface string `yaml:"interface,omitempty" json:"interface,omitempty"`
	CheckHost string `yaml:"check_host,omitempty" json:"check_host,omitempty"`
	CheckPort int    `yaml:"check_port,omitempty" json:"check_port,omitempty"`
}

type Build struct {
	Dockerfile string            `yaml:"dockerfile,omitempty" json:"dockerfile"`
	Context    string            `yaml:"context,omitempty" json:"context"`
	Target     string            `yaml:"target,omitempty" json:"target,omitempty"`
	Platform   string            `yaml:"platform,omitempty" json:"platform,omitempty"`
	Args       map[string]string `yaml:"args,omitempty" json:"args,omitempty"`
}

type Deploy struct {
	Strategy     string   `yaml:"strategy,omitempty" json:"strategy"`
	Path         string   `yaml:"path,omitempty" json:"path,omitempty"`
	Timeout      int      `yaml:"timeout,omitempty" json:"timeout"`
	Retain       int      `yaml:"retain,omitempty" json:"retain"`
	AutoRollback bool     `yaml:"auto_rollback,omitempty" json:"auto_rollback"`
	Hooks        Hooks    `yaml:"hooks,omitempty" json:"hooks"`
	BlueGreen    Blue     `yaml:"blue_green,omitempty" json:"blue_green"`
	Concurrency  int      `yaml:"concurrency,omitempty" json:"concurrency"`
	Swarm        Swarm    `yaml:"swarm,omitempty" json:"swarm"`
	Raw          []string `yaml:"-" json:"-"`
	ComposeFiles []string `yaml:"compose_files,omitempty" json:"compose_files,omitempty"`
	EnvFile      string   `yaml:"env_file,omitempty" json:"env_file,omitempty"`
	// PruneBuildCache controls cleanup of the local Docker build cache after a
	// successful deploy: "off" (never), "capped" (cap total cache size, keeps
	// recent layers for fast rebuilds) or "all" (wipe everything). Default "capped".
	PruneBuildCache string `yaml:"prune_build_cache,omitempty" json:"prune_build_cache,omitempty"`
	// BuildCacheKeep is the max build-cache size kept when PruneBuildCache is
	// "capped" (e.g. "5GB", "512MB"). Default "5GB".
	BuildCacheKeep string `yaml:"build_cache_keep,omitempty" json:"build_cache_keep,omitempty"`
}

type Hooks struct {
	PreDeploy  []string `yaml:"pre_deploy,omitempty" json:"pre_deploy"`
	PostDeploy []string `yaml:"post_deploy,omitempty" json:"post_deploy"`
	Pre        []string `yaml:"pre,omitempty" json:"-"`
	Post       []string `yaml:"post,omitempty" json:"-"`
}

type Blue struct {
	DrainTimeout   int `yaml:"drain_timeout,omitempty" json:"drain_timeout"`
	ReadinessDelay int `yaml:"readiness_delay,omitempty" json:"readiness_delay"`
}

type Swarm struct {
	Replicas int `yaml:"replicas,omitempty" json:"replicas"`
}

type Service struct {
	Port        int         `yaml:"port,omitempty" json:"port,omitempty"`
	Command     string      `yaml:"command,omitempty" json:"command"`
	Replicas    int         `yaml:"replicas,omitempty" json:"replicas,omitempty"`
	Healthcheck Healthcheck `yaml:"healthcheck,omitempty" json:"healthcheck"`
}

type Healthcheck struct {
	Type     string `yaml:"type,omitempty" json:"type"`
	Path     string `yaml:"path,omitempty" json:"path,omitempty"`
	Command  string `yaml:"command,omitempty" json:"command,omitempty"`
	Interval int    `yaml:"interval,omitempty" json:"interval"`
	Timeout  int    `yaml:"timeout,omitempty" json:"timeout"`
	Retries  int    `yaml:"retries,omitempty" json:"retries"`
}

type Accessory struct {
	Preset  string            `yaml:"preset,omitempty" json:"preset,omitempty"`
	Image   string            `yaml:"image,omitempty" json:"image,omitempty"`
	Port    any               `yaml:"port,omitempty" json:"port,omitempty"`
	Volumes []string          `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

type Secrets struct {
	File   string `yaml:"file,omitempty" json:"file"`
	Driver string `yaml:"driver,omitempty" json:"driver"`
}

type Env struct {
	Clear  map[string]string `yaml:"clear,omitempty" json:"clear,omitempty"`
	Secret []string          `yaml:"secret,omitempty" json:"secret,omitempty"`
}

type Proxy struct {
	Driver  string            `yaml:"driver,omitempty" json:"driver"`
	SSL     ProxySSL          `yaml:"ssl,omitempty" json:"ssl"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type ProxySSL struct {
	Provider string `yaml:"provider,omitempty" json:"provider"`
	Email    string `yaml:"email,omitempty" json:"email,omitempty"`
}

type Registry struct {
	Driver      string `yaml:"driver,omitempty" json:"driver"`
	URL         string `yaml:"url,omitempty" json:"url,omitempty"`
	Username    string `yaml:"username,omitempty" json:"username,omitempty"`
	PasswordEnv string `yaml:"password_env,omitempty" json:"password_env,omitempty"`
}

type Notifications struct {
	Webhooks []string `yaml:"webhooks,omitempty" json:"webhooks,omitempty"`
}

type Caddy struct {
	ConfDir       string            `yaml:"conf_dir,omitempty" json:"conf_dir,omitempty"`
	ReloadCommand string            `yaml:"reload_command,omitempty" json:"reload_command,omitempty"`
	Network       string            `yaml:"network,omitempty" json:"network,omitempty"`
	TLSSnippet    string            `yaml:"tls_snippet,omitempty" json:"tls_snippet,omitempty"`
	Routes        map[string]string `yaml:"routes,omitempty" json:"routes,omitempty"`
	Email         string            `yaml:"email,omitempty" json:"email,omitempty"`
	BasicAuth     CaddyBasicAuth    `yaml:"basic_auth,omitempty" json:"basic_auth,omitempty"`
}

type CaddyBasicAuth struct {
	Users []CaddyBasicAuthUser `yaml:"users,omitempty" json:"users,omitempty"`
}

type CaddyBasicAuthUser struct {
	Username string `yaml:"username" json:"username"`
	Hash     string `yaml:"hash" json:"hash"`
}

type Dev struct {
	SSL    bool     `yaml:"ssl,omitempty" json:"ssl"`
	Domain string   `yaml:"domain,omitempty" json:"domain,omitempty"`
	Ports  DevPorts `yaml:"ports,omitempty" json:"ports,omitempty"`
}

type DevPorts struct {
	HTTP  int `yaml:"http,omitempty" json:"http"`
	HTTPS int `yaml:"https,omitempty" json:"https"`
}

func Load(path string, env string) (*Config, error) {
	if path == "" {
		found, err := Find("shuttle.yml")
		if err != nil {
			return nil, err
		}
		path = found
	}

	cfg, err := loadFile(path)
	if err != nil {
		return nil, err
	}
	if env != "" {
		overlay := filepath.Join(filepath.Dir(path), fmt.Sprintf("shuttle.%s.yml", env))
		if _, err := os.Stat(overlay); err == nil {
			other, err := loadFile(overlay)
			if err != nil {
				return nil, err
			}
			merge(cfg, other)
		}
	}
	applyDefaults(cfg)
	return cfg, validate(cfg)
}

func Find(name string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found", name)
		}
		dir = parent
	}
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server != nil && len(cfg.Servers) == 0 {
		cfg.Servers = map[string]ServerGroup{
			"default": {Hosts: []string{cfg.Server.Host}, User: cfg.Server.User, Port: cfg.Server.Port, VPN: cfg.Server.VPN},
		}
	}
	for name, group := range cfg.Servers {
		if group.Port == 0 {
			group.Port = 22
		}
		if group.VPN.Required && group.VPN.CheckPort == 0 {
			group.VPN.CheckPort = group.Port
		}
		cfg.Servers[name] = group
	}
	if cfg.Build.Dockerfile == "" {
		cfg.Build.Dockerfile = "Dockerfile"
	}
	if cfg.Build.Context == "" {
		cfg.Build.Context = "."
	}
	if cfg.Deploy.Strategy == "" {
		cfg.Deploy.Strategy = "swarm"
	}
	if cfg.Deploy.Timeout == 0 {
		cfg.Deploy.Timeout = 120
	}
	if cfg.Deploy.Retain == 0 {
		cfg.Deploy.Retain = 5
	}
	if cfg.Deploy.Concurrency == 0 {
		cfg.Deploy.Concurrency = 5
	}
	if cfg.Deploy.PruneBuildCache == "" {
		cfg.Deploy.PruneBuildCache = "capped"
	}
	if cfg.Deploy.BuildCacheKeep == "" {
		cfg.Deploy.BuildCacheKeep = "5GB"
	}
	cfg.Deploy.AutoRollback = true
	// Normalize hook aliases: "pre"/"post" → "pre_deploy"/"post_deploy"
	if len(cfg.Deploy.Hooks.Pre) > 0 && len(cfg.Deploy.Hooks.PreDeploy) == 0 {
		cfg.Deploy.Hooks.PreDeploy = cfg.Deploy.Hooks.Pre
	}
	if len(cfg.Deploy.Hooks.Post) > 0 && len(cfg.Deploy.Hooks.PostDeploy) == 0 {
		cfg.Deploy.Hooks.PostDeploy = cfg.Deploy.Hooks.Post
	}
	if (cfg.Deploy.Strategy == "compose" || cfg.Deploy.Strategy == "swarm" || cfg.Deploy.Strategy == "blue-green") && len(cfg.Deploy.ComposeFiles) == 0 {
		cfg.Deploy.ComposeFiles = []string{"docker-compose.yml"}
	}
	if cfg.Proxy.Driver == "" {
		cfg.Proxy.Driver = "caddy"
	}
	if cfg.Secrets.Driver == "" {
		cfg.Secrets.Driver = "aes"
	}
	if cfg.Secrets.File == "" {
		cfg.Secrets.File = ".shuttle/secrets.enc"
	}
	if cfg.Caddy.ConfDir == "" {
		cfg.Caddy.ConfDir = "/opt/caddy/conf.d"
	}
	if cfg.Caddy.ReloadCommand == "" {
		cfg.Caddy.ReloadCommand = "docker service update --force caddy_caddy"
	}
	if cfg.Caddy.Network == "" {
		cfg.Caddy.Network = "caddy_network"
	}
	if cfg.Dev.Ports.HTTP == 0 {
		cfg.Dev.Ports.HTTP = 80
	}
	if cfg.Dev.Ports.HTTPS == 0 {
		cfg.Dev.Ports.HTTPS = 443
	}
}

func validate(cfg *Config) error {
	if cfg.App == "" {
		return errors.New("app is required")
	}
	if cfg.Domain == nil {
		return errors.New("domain is required")
	}
	if len(cfg.Servers) == 0 {
		return errors.New("server or servers is required")
	}
	if cfg.Deploy.Strategy != "blue-green" && cfg.Deploy.Strategy != "rolling" && cfg.Deploy.Strategy != "swarm" && cfg.Deploy.Strategy != "compose" {
		return fmt.Errorf("invalid deploy strategy %q", cfg.Deploy.Strategy)
	}
	for i, user := range cfg.Caddy.BasicAuth.Users {
		if user.Username == "" {
			return fmt.Errorf("caddy.basic_auth.users[%d].username is required", i)
		}
		if user.Hash == "" {
			return fmt.Errorf("caddy.basic_auth.users[%d].hash is required", i)
		}
	}
	return nil
}

func merge(dst *Config, src *Config) {
	rawDst, _ := json.Marshal(dst)
	rawSrc, _ := json.Marshal(src)
	var dstMap map[string]any
	var srcMap map[string]any
	_ = json.Unmarshal(rawDst, &dstMap)
	_ = json.Unmarshal(rawSrc, &srcMap)
	deepMerge(dstMap, srcMap)
	out, _ := json.Marshal(dstMap)
	_ = json.Unmarshal(out, dst)
}

func deepMerge(dst map[string]any, src map[string]any) {
	for key, srcVal := range src {
		if srcNested, ok := srcVal.(map[string]any); ok {
			if dstNested, ok := dst[key].(map[string]any); ok {
				deepMerge(dstNested, srcNested)
				continue
			}
		}
		dst[key] = srcVal
	}
}
