package main

import (
	"fmt"

	"github.com/Calgorr/EnglishPinglish/config"
	"github.com/Calgorr/EnglishPinglish/internal/handlers"
)

func main() {
	viper, err := config.LoadConfig("/app/config.yml")
	if err != nil {
		panic(err)
	}
	cfg, err := config.ParseConfig(viper)
	if err != nil {
		panic(err)
	}

	fmt.Println(cfg)
	server := handlers.NewServer(cfg)
	if err = server.Start(); err != nil {
		panic(err)
	}
}
