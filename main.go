package main

import (
	"io"
	"log"
	"os"
	"time"
)

func main() {
	f, _ := os.OpenFile("combcore.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer f.Close()
	wrt := io.MultiWriter(os.Stdout, f)
	log.SetOutput(wrt)

	var err error

	combcore_set_status("Initializing...")

	combcore_init()
	neominer_init()
	push_init()
	btc_init()

	if err = db_open(); err != nil {
		log.Fatal(err)
	}

	rpc_start()

	combcore_set_status("Loading...")
	db_start()
	combcore_set_status("Idle")

	for {
		if BTCInfo.Enabled {
			btc_sync()
		}

		if PushInfo.Enabled {
			push_sync()
		}

		combcore_set_status("Idle")
		time.Sleep(time.Second * 10)
	}
}
