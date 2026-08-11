package main

import (
	"flag"
	"fmt"
	"os"

	"net/http"

	"github.com/gorilla/pat"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"github.com/doctolib/MailHog/generated/assets"
	"github.com/doctolib/MailHog/pkg/api"
	"github.com/doctolib/MailHog/pkg/config"
	"github.com/doctolib/MailHog/pkg/smtp"
	"github.com/doctolib/MailHog/pkg/web"
)

var conf *config.Config
var exitCh chan int
var version string

func configure() {
	config.RegisterFlags()
	flag.Parse()
	conf = config.Configure()
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		log.Info("MailHog version: " + version)
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "bcrypt" {
		var pw string
		if len(os.Args) > 2 {
			pw = os.Args[2]
		} else {
			// TODO: read from stdin
			panic(fmt.Errorf("bcrypt command requires an argument"))
		}
		b, err := bcrypt.GenerateFromPassword([]byte(pw), 4)
		if err != nil {
			log.Fatalf("error bcrypting password: %s", err)
		}
		fmt.Println(string(b))
		os.Exit(0)
	}

	configure()

	if conf.CleanOnStart {
		if err := runCleanOnStart(conf); err != nil {
			log.Fatalf("cleanup failed: %s", err)
		}
		os.Exit(0)
	}

	if conf.AuthFile != "" {
		web.AuthFile(conf.AuthFile)
	}

	exitCh = make(chan int)
	cb := func(r http.Handler) {
		web.CreateWeb(conf, r.(*pat.Router), assets.Asset)
		api.CreateAPI(conf, r.(*pat.Router))
	}
	go web.Listen(conf.HTTPBindAddr, assets.Asset, exitCh, cb)
	go smtp.Listen(conf, exitCh)

	<-exitCh
	log.Printf("Received exit signal")
}

// runCleanOnStart deletes all messages from the configured storage backend.
// Used to run MailHog as a one-shot cleanup job (e.g. from a cronjob) instead
// of starting the SMTP/HTTP listeners.
func runCleanOnStart(conf *config.Config) error {
	log.Info("MH_CLEAN_ON_START set: deleting all messages and exiting")
	if err := conf.Storage.DeleteAll(); err != nil {
		return err
	}
	log.Info("cleanup complete")
	return nil
}
