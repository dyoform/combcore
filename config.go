package main

import (
	"flag"
)

var (
	btc_peer = flag.String("btc_peer", "", "")
	btc_port = flag.Uint("btc_port", 8332, "")
	btc_data = flag.String("btc_data", "", "")

	comb_host    = flag.String("comb_host", "127.0.0.1", "")
	comb_port    = flag.Uint("comb_port", 2211, "")
	comb_network = flag.String("comb_network", "mainnet", "")

	push_ip   = flag.String("push_client_ip", "", "")
	push_port = flag.Uint("push_client_port", 2211, "")
)
