package main

import (
	"time"
)

func main() {
	set_log_file("combcore.log")

	var err error

	combcore_set_status("Initializing...")

	combcore_init()
	ingest_init()
	push_init()
	btc_init()

	if err = db_open(); err != nil {
		log_panic("db", "failed to open (%s)", err.Error())
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
