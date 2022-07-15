package main

import (
	"encoding/binary"
	"libcomb"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/vharitonsky/iniflags"
)

var critical sync.Mutex
var shutdown sync.Mutex

var COMBInfo struct {
	Height uint64
	Hash   [32]byte
	Chain  map[[32]byte][32]byte //child -> parent

	Network string
	Magic   uint32
	Prefix  map[string]string
	Path    string

	Guard sync.RWMutex
}

func setup_graceful_shutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		LogStatus("combcore", "terminate signal detected. shutting down...")
		critical.Lock()
		db.Close()
		shutdown.Unlock()
		os.Exit(-3)
	}()
	shutdown.Lock()
}

func combcore_init() {
	iniflags.SetAllowMissingConfigFile(false)
	iniflags.SetAllowUnknownFlags(false)
	iniflags.SetConfigFile("config.ini")
	iniflags.Parse()

	//reset to known empty state
	libcomb.Reset()

	combcore_set_network()

	setup_graceful_shutdown()
}

func combcore_set_network() {
	COMBInfo.Guard.Lock()
	defer COMBInfo.Guard.Unlock()

	COMBInfo.Prefix = make(map[string]string)
	COMBInfo.Chain = make(map[[32]byte][32]byte)

	COMBInfo.Network = *comb_network

	LogStatus("combcore", "loading in %s mode", COMBInfo.Network)

	//every difference between the networks is here (minus whats in libcomb)
	switch COMBInfo.Network {
	case "mainnet":
		COMBInfo.Height = 481822
		COMBInfo.Hash, _ = parse_hex("0000000000000000003bec88b7ba0bebd8eb3b1c1c599e44a2b270ad3e8203ca")
		COMBInfo.Magic = binary.LittleEndian.Uint32([]byte{0xf9, 0xbe, 0xb4, 0xd9})
		COMBInfo.Path = "commits"
		COMBInfo.Prefix["stack"] = "/stack/data/"
		COMBInfo.Prefix["tx"] = "/tx/recv/"
		COMBInfo.Prefix["key"] = "/wallet/data/"
		COMBInfo.Prefix["merkle"] = "/merkle/data/"
		COMBInfo.Prefix["unsigned_merkle"] = "/contract/data/"
		COMBInfo.Prefix["decider"] = "/purse/data/"
	case "testnet":
		COMBInfo.Height = 0
		COMBInfo.Hash, _ = parse_hex("000000000933ea01ad0ee984209779baaec3ced90fa3f408719526f8d77f4943")
		COMBInfo.Magic = binary.LittleEndian.Uint32([]byte{0x0B, 0x11, 0x09, 0x07})
		COMBInfo.Path = "commits_testnet"
		COMBInfo.Prefix["stack"] = "\\stack\\data\\"
		COMBInfo.Prefix["tx"] = "\\tx\\recv\\"
		COMBInfo.Prefix["key"] = "\\wallet\\data\\"
		COMBInfo.Prefix["merkle"] = "\\merkle\\data\\"
		COMBInfo.Prefix["unsigned_merkle"] = "\\contract\\data\\"
		COMBInfo.Prefix["decider"] = "\\purse\\data\\"
		libcomb.SwitchToTestnet()
	default:
		LogPanic("combcore", "unknown network %s", COMBInfo.Network)
	}

	libcomb.SetHeight(COMBInfo.Height)
	COMBInfo.Chain[COMBInfo.Hash] = [32]byte{}
}

func combcore_process_block(block Block) (err error) {
	COMBInfo.Guard.Lock()
	defer COMBInfo.Guard.Unlock()

	if block.Metadata.Hash == [32]byte{} {
		return //discard dummy blocks
	}

	if !DBInfo.InitialLoad {
		LogInfo("combcore", "processing %d", block.Metadata.Height)
	}

	if block.Metadata.Previous != COMBInfo.Hash { //sanity check
		LogError("combcore", "%d %X %d %X (%X)", COMBInfo.Height, COMBInfo.Hash, block.Metadata.Height, block.Metadata.Hash, block.Metadata.Previous)
		LogError("combcore", "sanity check failed, chain is broken")
	}

	var lib_block libcomb.Block
	lib_block.Commits = block.Commits

	libcomb.GetLock() //would be more efficient to load in batches
	libcomb.LoadBlock(lib_block)
	libcomb.ReleaseLock()

	COMBInfo.Height = libcomb.GetHeight()
	if COMBInfo.Height != block.Metadata.Height { //sanity check
		LogError("combcore", "%d %d %X\n", COMBInfo.Height, block.Metadata.Height, block.Metadata.Hash)
		LogError("combcore", "sanity check failed, height mismatch")
	}
	COMBInfo.Chain[block.Metadata.Hash] = COMBInfo.Hash
	COMBInfo.Hash = block.Metadata.Hash
	return nil
}

func combcore_reorg(target [32]byte) {
	COMBInfo.Guard.Lock()
	defer COMBInfo.Guard.Unlock()

	//target is the highest common block between our chain and the new reorged chain
	//this function should remove all block data after target, and rollback libcomb to target
	var ok bool
	var metadata = db_get_block_metadata_by_hash(target)

	LogStatus("combcore", "reorg encountered, rolling back to block %d", metadata.Height)

	LogStatus("combcore", "tracing back...")
	//trace back our in-memory chain
	for COMBInfo.Hash != target {
		if COMBInfo.Hash, ok = COMBInfo.Chain[COMBInfo.Hash]; !ok {
			LogPanic("combcore", "reorg past checkpoint is not possible")
		}
	}

	LogStatus("combcore", "removing blocks from database...")
	//remove reorg'd blocks from the db
	db_remove_blocks_after(metadata.Height + 1)

	LogStatus("combcore", "unloading blocks...")
	//unload libcomb to the target height
	libcomb.GetLock()
	for COMBInfo.Height != metadata.Height {
		COMBInfo.Height = libcomb.UnloadBlock()
	}
	libcomb.FinishReorg()
	libcomb.ReleaseLock()

	LogStatus("combcore", "finished at %X (%d)", COMBInfo.Hash, COMBInfo.Height)
}
