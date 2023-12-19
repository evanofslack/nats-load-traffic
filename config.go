package main

import (
	"fmt"
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

func loadFromString(s string) (load, error) {
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

const defaultLoad = Constant

type Nats struct {
	Url string `yaml:"url"`
}

type RemoteWrite struct {
	Enabled bool    `yaml:"enabled"`
	Url     *string `yaml:"url"`
}

type Profile struct {
	Name     string        `yaml:"name"`
	Subject  string        `yaml:"subject"`
	Load     string        `yaml:"load"`
	Rate     int           `yaml:"rate"`
	Duration time.Duration `yaml:"duration"`
}

type Config struct {
	RemoteWrite RemoteWrite `yaml:"remote_write"`
	Nats        Nats        `yaml:"nats"`
	Profiles    []Profile   `yaml:"profiles"`
}

func newConfig(configPath string) (*Config, error) {
	config := &Config{}

	fmt.Printf("loading config from %s\n", configPath)
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	fmt.Printf("loaded config with %d profiles\n", len(config.Profiles))
	fmt.Printf("nats url: %s\n", config.Nats.Url)
	wr := config.RemoteWrite
	fmt.Printf("remote write: %t\n", wr.Enabled)
	if wr.Url != nil {
		fmt.Printf("remote write url: %s\n", *wr.Url)
	}

	return config, nil
}
