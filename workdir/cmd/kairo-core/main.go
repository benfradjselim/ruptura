package main

import (
	"flag"
	"log"
)

func main() {
	configPath := flag.String("config", "/etc/kairo/kairo.yaml", "path to config file")
	flag.Parse()
	log.Printf("kairo-core starting, config=%s", *configPath)
}
