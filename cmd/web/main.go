package main

import (
	"log"

	"github.com/Sourjaya/converse/app/server"
)

func main() {
	log.Println("WASM Server Starting...")
	server.Start()
}
