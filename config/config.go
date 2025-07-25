package config

import (
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Redis  RedisConfig  `yaml:"redis"`
	Ninja  NinjaConfig  `yaml:"ninja"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type RedisConfig struct {
	Addr string `yaml:"addr"`
	TTL  int    `yaml:"ttl"`
}

type NinjaConfig struct {
	NinjaAPIKey        string `yaml:"ninjaapikey"`
	NinjaDictionaryURL string `yaml:"ninjadictionaryurl"`
	NinjaRandomURL     string `yaml:"ninjarandomurl"`
}

func LoadConfig(filename string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(filename)
	v.SetConfigType("yml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v, nil
}

func ParseConfig(v *viper.Viper) (*Config, error) {
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	if apiKey := os.Getenv("Ninja_API_KEY"); apiKey != "" {
		cfg.Ninja.NinjaAPIKey = apiKey
	}
	return cfg, nil
}
