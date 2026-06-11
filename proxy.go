package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type proxyHandler struct {
	resolver *net.Resolver
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		host, port, err := net.SplitHostPort(r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		ips, err := p.resolver.LookupIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			http.Error(w, "DNS lookup failed", http.StatusBadGateway)
			return
		}

		targetAddr := net.JoinHostPort(ips[0].String(), port)
		destConn, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}

		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

		go func() {
			io.Copy(destConn, clientConn)
			destConn.Close()
		}()
		go func() {
			io.Copy(clientConn, destConn)
			clientConn.Close()
		}()
		return
	}

	// Handle plain HTTP
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, _ := net.SplitHostPort(addr)
			ips, err := p.resolver.LookupIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, err
			}
			return net.DialTimeout("tcp", net.JoinHostPort(ips[0].String(), port), 10*time.Second)
		},
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	r.RequestURI = ""
	resp, err := client.Do(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func startDNSProxy(dnsServer string) (string, error) {
	if !strings.Contains(dnsServer, ":") {
		dnsServer = net.JoinHostPort(dnsServer, "53")
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", dnsServer)
		},
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go http.Serve(l, &proxyHandler{resolver: resolver})

	return "http://" + l.Addr().String(), nil
}
