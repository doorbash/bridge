package main

import (
	"context"
	"fmt"
	"time"

	"github.com/doorbash/bridge/adapter/outbound"
	"github.com/doorbash/bridge/log"
)

func main() {
	log.Info("this is ssr test")

	ssrNode := make(map[string]interface{})
	ssrNode["name"] = "ssr"
	ssrNode["type"] = "ssr"
	ssrNode["server"] = "1.2.3.4"
	ssrNode["port"] = 11800
	ssrNode["password"] = "1234"
	ssrNode["cipher"] = "aes-256-cfb"
	ssrNode["obfs"] = "http_simple"
	ssrNode["obfs-param"] = ""
	ssrNode["protocol"] = "origin"
	ssrNode["protocol-param"] = ""
	ssrNode["udp"] = true

	p, err := outbound.ParseProxy(ssrNode)

	socks5Node := make(map[string]interface{})
	socks5Node["name"] = "socks"
	socks5Node["type"] = "socks5"
	socks5Node["server"] = "127.0.0.1"
	socks5Node["port"] = 8080
	socks5Node["udp"] = true
	socks5Node["skip-cert-verify"] = true

	ps, _ := outbound.ParseProxy(socks5Node)

	p.SetDialer(ps)

	if err != nil {
		fmt.Println(err)
	}

	url := "https://api.ipify.org/"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	body, latency, err := p.URLTest(ctx, url)
	if err != nil {
		panic(err)
	}

	log.Info("response: %s\nlatency: %d ms", body, latency)

}
