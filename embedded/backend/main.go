package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

var (
	port = 8080
)

func main() {
	r := httprouter.New()

	// DO NOT CHANGE, WHERE THE SVELTE CODE GETS REGISTERED
	registerWAPGen(r)

	err := http.ListenAndServe(":"+strconv.Itoa(port), r)
	if err != nil {
		log.Fatalf("Couldn't start http server on port %d, error: %v", port, err)
	}
}
