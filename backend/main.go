package main

import (
	"log"
	"net/http"
)

func main() {
	app, err := newApp()
	if err != nil {
		log.Fatal(err)
	}
	defer app.db.Close()

	addr := ":" + env("PORT", defaultPort)
	log.Printf("Agenda Monitor running on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, logRequest(app.routes())))
}
