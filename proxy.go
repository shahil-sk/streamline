package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type proxyHandler struct {
	resolver *net.Resolver
	dohURL   string
}

func (p *proxyHandler) lookupIP(ctx context.Context, host string) ([]net.IP, error) {
	if p.dohURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", p.dohURL+"?name="+host+"&type=A", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/dns-json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var res struct {
			Answer []struct {
				Data string `json:"data"`
				Type int    `json:"type"`
			} `json:"Answer"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}

		var ips []net.IP
		for _, a := range res.Answer {
			if a.Type == 1 || a.Type == 28 {
				if ip := net.ParseIP(a.Data); ip != nil {
					ips = append(ips, ip)
				}
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no IPs found via DoH")
		}
		return ips, nil
	}
	return p.resolver.LookupIP(ctx, "ip", host)
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

		ips, err := p.lookupIP(ctx, host)
		if err != nil || len(ips) == 0 {
			http.Error(w, "DNS lookup failed", http.StatusBadGateway)
			return
		}

		var destConn net.Conn
		for _, ip := range ips {
			targetAddr := net.JoinHostPort(ip.String(), port)
			destConn, err = net.DialTimeout("tcp", targetAddr, 5*time.Second)
			if err == nil {
				break
			}
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}

		clientConn, bufrw, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		bufrw.Flush()

		go func() {
			io.Copy(destConn, bufrw)
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
			ips, err := p.lookupIP(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, err
			}
			var destConn net.Conn
			for _, ip := range ips {
				destConn, err = net.DialTimeout("tcp", net.JoinHostPort(ip.String(), port), 5*time.Second)
				if err == nil {
					return destConn, nil
				}
			}
			return nil, err
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
	var handler *proxyHandler

	if strings.HasPrefix(dnsServer, "https://") {
		handler = &proxyHandler{dohURL: dnsServer}
	} else {
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
		handler = &proxyHandler{resolver: resolver}
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go http.Serve(l, handler)

	return "http://" + l.Addr().String(), nil
}
