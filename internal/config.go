package internal

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type load int

const (
	Constant load = iota
	Periodic
	Rare
	Random
	Unknown
)

func (l load) String() string {
	switch l {
	case Constant:
		return "constant"
	case Periodic:
		return "periodic"
	case Rare:
		return "rare"
	case Random:
		return "random"
	}
	return "unknown"
}

func LoadFromString(s string) (load, error) {
	switch strings.ToLower(s) {
	case "constant":
		return Constant, nil
	case "periodic":
		return Periodic, nil
	case "rare":
		return Rare, nil
	case "random":
		return Random, nil
	}
	return Unknown, fmt.Errorf("unknown load type: %s", s)
}

const DefaultLoad = Constant

type Nats struct {
	Url string `yaml:"url"`
}

type RemoteWrite struct {
	Enabled  bool           `yaml:"enabled"`
	Url      *string        `yaml:"url"`
	Interval *time.Duration `yaml:"interval"`
}

type Profile struct {
	Name     string        `yaml:"name"`
	Subject  string        `yaml:"subject"`
	Load     string        `yaml:"load"`
	Rate     int           `yaml:"rate"`
	Duration time.Duration `yaml:"duration"`
}

type Log struct {
	Level string `yaml:"level"`
	Env   string `yaml:"env"`
}

type Config struct {
	Log         Log         `yaml:"log"`
	RemoteWrite RemoteWrite `yaml:"remote_write"`
	Nats        Nats        `yaml:"nats"`
	Profiles    []Profile   `yaml:"profiles"`
}

func NewConfig(configPath string) (*Config, error) {
	config := &Config{}

	slog.Info(fmt.Sprintf("loading config from %s", configPath))
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}
	return config, nil
}
