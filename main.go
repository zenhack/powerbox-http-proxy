package main

import (
	"database/sql"
	"encoding/pem"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

// Get the named enviornment variable, aborting with an error if it is undefined.
func mustGetenv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("Error: environment variable %q is not defined.", name)
	}
	return value
}

var (
	webSocketListenAddr = ":" + mustGetenv("POWERBOX_WEBSOCKET_PORT")
	proxyListenAddr     = ":" + mustGetenv("POWERBOX_PROXY_PORT")

	caCertFile = mustGetenv("CA_CERT_PATH")

	dbType = mustGetenv("DB_TYPE")
	dbUri  = mustGetenv("DB_URI")
)

func chkfatal(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	caspoof, err := GenSpoofer()
	chkfatal(err)

	func() {
		f, err := os.Create(caCertFile)
		chkfatal(err)
		defer f.Close()
		chkfatal(pem.Encode(f, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caspoof.RawCACert(),
		}))
	}()

	db, err := sql.Open(dbType, dbUri)
	chkfatal(err)
	storage, err := NewStorage(db)
	chkfatal(err)
	srv := NewServer(storage, caspoof)

	go func() {
		panic(http.ListenAndServe(webSocketListenAddr, srv.WebSocketHandler()))
	}()
	panic(http.ListenAndServe(proxyListenAddr, srv.ProxyHandler()))
}
