package main

import (
	"github.com/syndtr/goleveldb/leveldb"
)

var IngestInfo struct {
	BatchCapacity uint64
	BatchCached   uint64
	Batch         *leveldb.Batch
}

func ingest_init() {
	IngestInfo.BatchCapacity = 10000
	IngestInfo.BatchCached = 0
	IngestInfo.Batch = new(leveldb.Batch)
}

func ingest_write() {
	if err := db_write(IngestInfo.Batch); err != nil {
		LogPanic("ingest", "write batch failed (%s)", err.Error())
		return
	}
	IngestInfo.BatchCached = 0
}

func ingest_process_block(block_data BlockData) (reorg bool) {
	var err error
	var block Block
	block.Metadata.Hash = block_data.Hash
	block.Metadata.Previous = block_data.Previous
	block.Commits = block_data.Commits
	block.Metadata.Fingerprint = db_compute_block_fingerprint(block.Commits)

	//check if we already have this block
	if _, ok := COMBInfo.Chain[block.Metadata.Hash]; ok {
		LogInfo("ingest", "block discarded %X", block.Metadata.Hash)
		return
	}

	//check that we have the previous block
	if _, ok := COMBInfo.Chain[block.Metadata.Previous]; !ok {
		LogPanic("ingest", "chain broken, mining has fucked up %X, %X", block.Metadata.Hash, block.Metadata.Previous)
	}

	//if the previous block isnt the top block its a reorg
	if block.Metadata.Previous != COMBInfo.Hash {
		//flush the cache so we dont write back reorg'd blocks
		ingest_write()

		//remove all the blocks after previous in the chain
		combcore_reorg(block.Metadata.Previous)

		//the previous block should now be the top block
		if block.Metadata.Previous != COMBInfo.Hash {
			LogPanic("ingest", "reorg failed! %X != %X", block.Metadata.Previous, COMBInfo.Hash)
		}
	}

	//now process this block

	block.Metadata.Height = COMBInfo.Height + 1

	//this doesnt touch the disk yet, just gets added to the current batch
	if err = db_process_block(IngestInfo.Batch, block); err != nil {
		LogPanic("ingest", "store block failed (%s)", err.Error())
		return
	}
	IngestInfo.BatchCached++
	if err = combcore_process_block(block); err != nil {
		LogPanic("ingest", "process block failed (%s)", err.Error())
	}

	if IngestInfo.BatchCached >= IngestInfo.BatchCapacity {
		ingest_write()
	}

	return false
}
