package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type BlockData struct {
	Hash     [32]byte
	Previous [32]byte
	Commits  [][32]byte
}

type ChainInfo struct {
	Height      uint64
	KnownHeight uint64
	TopHash     [32]byte
}

var BTCInfo struct {
	RestClient *http.Client
	RestURL    string
	DirectPath string
	Chain      ChainInfo
	Enabled    bool
	Guard      sync.RWMutex
}

func btc_init() {
	BTCInfo.Enabled = *btc_peer != ""
	if !BTCInfo.Enabled {
		log.Printf("(btc) mining disabled (no peer configured)")
		return
	}

	BTCInfo.RestURL = fmt.Sprintf("http://%s:%d/rest", *btc_peer, *btc_port)
	BTCInfo.RestClient = &http.Client{}

	if err := direct_check_path(*btc_data); err != nil {
		log.Printf("(btc) direct mining disabled (%s)\n", err.Error())
	} else {
		BTCInfo.DirectPath = *btc_data
	}
}

func btc_get_chains() {
	var err error
	BTCInfo.Guard.Lock()
	defer BTCInfo.Guard.Unlock()

	var chain ChainInfo
	if chain, err = rest_get_chains(BTCInfo.RestClient, BTCInfo.RestURL); err != nil {
		log.Printf("(btc) failed to get chains (%s)\n", err.Error())
		BTCInfo.Chain.KnownHeight = 0 //signals we are disconnected
		return
	}
	BTCInfo.Chain = chain
}

func btc_sync() {
	//sync chain info with our BTC peer
	btc_get_chains()

	if COMBInfo.Hash == BTCInfo.Chain.TopHash {
		return //nothing to do
	}

	//get block delta for displaying mining progress to the user
	var delta int64 = int64(BTCInfo.Chain.Height) - int64(COMBInfo.Height)
	log.Printf("(btc) %d blocks behind...\n", delta)

	var blocks chan BlockData = make(chan BlockData)
	var wait sync.Mutex
	wait.Lock()

	//spin up a goroutine to ingest blocks
	go func() {
		for block := range blocks {
			neominer_process_block(block)
		}
		//block channel closed, now flush the cache
		neominer_write()
		wait.Unlock()
	}()

	var target [32]byte = BTCInfo.Chain.TopHash
	if err := btc_get_block_range(target, uint64(delta), blocks); err != nil {
		log.Printf("(btc) failed to get blocks (%s)\n", err.Error())
	}
	wait.Lock() //dont leave before neominer is finished (only a problem if we use a buffered channel)
}

func btc_get_block_range(target [32]byte, delta uint64, blocks chan<- BlockData) (err error) {
	if BTCInfo.DirectPath != "" && delta > 10 { //use direct mining if its available and delta is big enough (>10)
		if err = direct_get_block_range(BTCInfo.DirectPath, target, delta, blocks); err != nil {
			return err
		}
	} else {
		if err = rest_get_block_range(BTCInfo.RestClient, BTCInfo.RestURL, target, delta, blocks); err != nil {
			return err
		}
	}
	return nil
}
func btc_parse_varint(data []byte) (value uint64, advance uint8) {
	//parse a BTC varint. see https://learnmeabitcoin.com/technical/varint

	prefix := data[0]

	switch prefix {
	case 0xfd:
		value = uint64(binary.LittleEndian.Uint16(data[1:]))
		advance = 3
	case 0xfe:
		value = uint64(binary.LittleEndian.Uint32(data[1:]))
		advance = 5
	case 0xff:
		value = uint64(binary.LittleEndian.Uint64(data[1:]))
		advance = 9
	default:
		value = uint64(prefix)
		advance = 1
	}

	return value, advance
}

func btc_parse_block(data []byte, block *BlockData) {
	//parse a raw BTC block. see https://learnmeabitcoin.com/technical/blkdat

	var current_commit [32]byte
	block.Hash = sha256.Sum256(data[0:80])
	block.Hash = sha256.Sum256(block.Hash[:])
	data = data[4:] //version(4)
	copy(block.Previous[:], data[0:32])
	data = data[32:] //previous(32)
	data = data[44:] //merkle root(32),time(4),bits(4),nonce(4)

	tx_count, adv := btc_parse_varint(data[:])
	data = data[adv:] //tx count(var)

	var segwit bool
	for t := 0; t < int(tx_count); t++ {
		segwit = false
		data = data[4:] //version(4)
		in_count, adv := btc_parse_varint(data[:])

		if in_count == 0 { //segwit marker is 0x00
			segwit = true
			data = data[2:] //marker(1),flag(1)
			in_count, adv = btc_parse_varint(data[:])
		}

		data = data[adv:] //vin count(var)
		for i := 0; i < int(in_count); i++ {
			data = data[36:] //txid(32), vout(4)
			sig_size, adv := btc_parse_varint(data[:])
			data = data[uint64(adv)+sig_size+4:] //sig size(var), sig(var),sequence(4)
		}
		out_count, adv := btc_parse_varint(data[:])
		data = data[adv:] //vout count(var)
		for i := 0; i < int(out_count); i++ {
			data = data[8:] //value(8)
			pub_size, adv := btc_parse_varint(data[:])
			data = data[uint64(adv):] //pub size(var)
			if pub_size == 34 && data[0] == 0 && data[1] == 32 {
				copy(current_commit[:], data[2:34])
				block.Commits = append(block.Commits, current_commit)
			}
			data = data[pub_size:] //pub (var)
		}
		if segwit {
			for i := 0; i < int(in_count); i++ {
				witness_count, adv := btc_parse_varint(data[:])
				data = data[adv:] //witness count(var)
				for w := 0; w < int(witness_count); w++ {
					witness_size, adv := btc_parse_varint(data[:])
					data = data[uint64(adv)+witness_size:] //witness size(var), witness(var)
				}
			}
		}
		data = data[4:] //locktime(4)
	}

	//we use big endian in haircomb, btc uses little endian for some values
	block.Hash = swap_endian(block.Hash)
	block.Previous = swap_endian(block.Previous)
}
