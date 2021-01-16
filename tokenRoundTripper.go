package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"

	"zenhack.net/go/sandstorm/capnp/grain"
	bridge "zenhack.net/go/sandstorm/capnp/sandstormhttpbridge"
	"zombiezen.com/go/capnproto2/rpc"
)

type tokenRoundTripper struct {
	token      string
	underlying http.RoundTripper
	url        string
	server     Server
}

func (tr *tokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := tr.tryRoundTrip(req)
	if err == nil && resp.StatusCode >= 500 {
		// It is possible that the token has been revoked; before we give up,
		// try to fetch a new one.
		if tr.tryRefreshToken(req.Context()) {
			// Refreshing the token succeeded; try the request again.
			// This time, if it fails it fails.
			return tr.tryRoundTrip(req)
		}
	}
	return resp, err
}

func (tr *tokenRoundTripper) tryRoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+tr.token)

	// Avoid trying to use TLS ourselves, as the bridge doesn't support CONNECT.
	// it will ignore our host & protocol anyway, as it just looks at the
	// Authorization header.
	req.URL.Scheme = "http"

	return tr.underlying.RoundTrip(req)
}

func (tr *tokenRoundTripper) tryRefreshToken(ctx context.Context) bool {
	ok, err := isValidToken(ctx, tr.token)
	if err != nil {
		log.Print("powerbox-http-proxy: Failed to check token validity: ", err)
	}
	if ok {
		return false
	}

	err = tr.server.storage.DeleteToken(tr.token)
	if err != nil {
		log.Print("powerbox-http-proxy: Failed to delete token: ", err)
		return false
	}
	token, err := tr.server.getTokenFor(tr.url)
	if err != nil {
		return false
	}
	tr.token = token
	return true
}

func isValidToken(ctx context.Context, token string) (bool, error) {
	tokenBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return false, fmt.Errorf("Failed to decode token: %w", err)
	}

	netConn, err := net.Dial("unix", "/tmp/sandstorm-api")
	if err != nil {
		return false, err
	}
	defer netConn.Close()
	rpcConn := rpc.NewConn(rpc.NewStreamTransport(netConn), nil)
	defer rpcConn.Close()

	bridgeClient := bridge.SandstormHttpBridge{Client: rpcConn.Bootstrap(ctx)}
	apiFut, release := bridgeClient.GetSandstormApi(ctx, nil)
	restoreFut, release := apiFut.Api().Restore(ctx, func(p grain.SandstormApi_restore_Params) error {
		p.SetToken(tokenBytes)
		return nil
	})
	defer release()
	_, err = restoreFut.Struct()
	return err == nil, nil
}
