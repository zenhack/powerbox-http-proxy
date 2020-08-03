package main

import (
	"database/sql"
	"encoding/pem"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var (
	webSocketListenAddr = ":" + os.Getenv("POWERBOX_WEBSOCKET_PORT")
	proxyListenAddr     = ":" + os.Getenv("POWERBOX_PROXY_PORT")

	caCertFile = os.Getenv("CA_CERT_PATH")

	dbType = os.Getenv("DB_TYPE")
	dbUri  = os.Getenv("DB_URI")
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

	db, err := sql.Open(dbType, mysqlUri)
	chkfatal(err)
	storage, err := NewStorage(db)
	chkfatal(err)
	srv := NewServer(storage, caspoof)

	go func() {
		panic(http.ListenAndServe(webSocketListenAddr, srv.WebSocketHandler()))
	}()
	panic(http.ListenAndServe(proxyListenAddr, srv.ProxyHandler()))
}
