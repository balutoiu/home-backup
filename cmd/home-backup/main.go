package main

import (
	"flag"
	"os"

	"github.com/balutoiu/home-backup/pkg/backup"
	"github.com/balutoiu/home-backup/pkg/config"

	log "github.com/sirupsen/logrus"
)

func initLogging(loggingLevel string) error {
	lvl, err := log.ParseLevel(loggingLevel)
	if err != nil {
		return err
	}
	log.SetOutput(os.Stdout)
	log.SetLevel(lvl)
	return nil
}

func main() {
	if os.Geteuid() != 0 {
		log.Fatalf("this script must be run as root")
	}

	loggingLevel := flag.String("log-level", "info", "Logging level")
	configPath := flag.String("config", "", "Path to the configuration file")
	flag.Parse()

	if err := initLogging(*loggingLevel); err != nil {
		log.Fatalf("error initializing logging: %v\n", err)
	}

	if *configPath == "" {
		log.Fatalf("usage: backupctl -config <config-path>")
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("error loading config file: %v\n", err)
	}

	if err := backup.CreateBackups(cfg); err != nil {
		log.Fatalf("error performing backups: %v\n", err)
	}
}
