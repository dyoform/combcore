package main

import (
	"log"

	"github.com/syndtr/goleveldb/leveldb"
)

var NeoInfo struct {
	BatchCapacity uint64
	BatchCached   uint64
	Batch         *leveldb.Batch
}

func neominer_inspect() {
	log.Println("BTC Node:")
	log.Printf("\tHeight: %d of %d\n", BTCInfo.Chain.Height, BTCInfo.Chain.KnownHeight)
	log.Println("COMB Node:")
	log.Printf("\tHeight: %d\n", COMBInfo.Height)
}

func neominer_init() {
	NeoInfo.BatchCapacity = 10000
	NeoInfo.BatchCached = 0
	NeoInfo.Batch = new(leveldb.Batch)
}

func neominer_write() {
	if err := db_write(NeoInfo.Batch); err != nil {
		log.Panicf("(neominer) write batch failed (%s)\n", err.Error())
		return
	}
	NeoInfo.BatchCached = 0
}

func neominer_process_block(block_data BlockData) (reorg bool) {
	var err error
	var block Block
	block.Metadata.Hash = block_data.Hash
	block.Metadata.Previous = block_data.Previous
	block.Commits = block_data.Commits
	block.Metadata.Fingerprint = db_compute_block_fingerprint(block.Commits)

	if block_data.Previous != COMBInfo.Hash { //reorg!
		if _, ok := COMBInfo.Chain[block_data.Previous]; !ok { //check we actually have the previous block in the chain
			log.Panicf("(neominer) chain broken, mining has fucked up\n")
		}
		neominer_write() //flush the cache so we dont write back reorg'd blocks
		combcore_reorg(block_data.Previous)
	}

	//check the rollback was successful
	if block_data.Previous != COMBInfo.Hash {
		log.Panicf("(neominer) reorg failed!")
	}

	block.Metadata.Height = COMBInfo.Height + 1

	if err = db_process_block(NeoInfo.Batch, block); err != nil {
		log.Panicf("(neominer) ingest store block failed (%s)\n", err.Error())
		return
	}
	NeoInfo.BatchCached++
	if err = combcore_process_block(block); err != nil {
		log.Panicf("(neominer) ingest process block failed (%s)\n", err.Error())
	}

	if NeoInfo.BatchCached >= NeoInfo.BatchCapacity {
		neominer_write()
	}

	return false
}
