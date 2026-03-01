package main

import (
	"log"
	"os"

	"github.com/pebbe/zmq4"
)

func main() {
	xsubBind := os.Getenv("XSUB_BIND_ADDR")
	if xsubBind == "" {
		xsubBind = "tcp://*:5556"
	}
	xpubBind := os.Getenv("XPUB_BIND_ADDR")
	if xpubBind == "" {
		xpubBind = "tcp://*:5557"
	}

	ctx, err := zmq4.NewContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Term()

	xsub, err := ctx.NewSocket(zmq4.XSUB)
	if err != nil {
		log.Fatal(err)
	}
	defer xsub.Close()

	xpub, err := ctx.NewSocket(zmq4.XPUB)
	if err != nil {
		log.Fatal(err)
	}
	defer xpub.Close()

	if err := xpub.SetXpubVerbose(1); err != nil {
		log.Printf("xpub verbose setup error: %v", err)
	}

	if err := xsub.Bind(xsubBind); err != nil { // publishers connect here
		log.Fatal(err)
	}
	if err := xpub.Bind(xpubBind); err != nil { // subscribers connect here
		log.Fatal(err)
	}

	log.Printf("broker up: XSUB %s, XPUB %s", xsubBind, xpubBind)
	if err := zmq4.Proxy(xsub, xpub, nil); err != nil {
		log.Fatal(err)
	}
}
