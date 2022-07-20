package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

type RawData map[[32]byte]*BlockData

func direct_parse_block_file(data []byte, blocks *RawData, path string) {
	f, err := os.Open(path)
	if err != nil {
		log_error("direct", "cant open file %s (%s)", path, err.Error())
		return
	}
	stats, _ := f.Stat()
	data = data[:stats.Size()]
	buf := bufio.NewReader(f)
	buf.Read(data)
	f.Close()

	var p int = 0
	var size int

	for {

		//check for the end of file, otherwise check if the magic bytes are there
		if p >= len(data) || binary.LittleEndian.Uint32(data[p:p+4]) != COMBInfo.Magic {
			break
		}
		p += 4
		//next 4 bytes is the size of the upcoming block
		size = int(binary.LittleEndian.Uint32(data[p : p+4]))
		p += 4

		//now actually parse the block
		block := new(BlockData)
		btc_parse_block(data[p:p+size], block)
		(*blocks)[block.Hash] = block
		p += size
	}
}

func direct_trace_chain(blocks *RawData, target [32]byte, length uint64) (block_chain [][32]byte) {
	//trace back from target to a known block (any block in COMBInfo.Chain)
	var hash [32]byte = target

	//lock the chain while we trace it
	COMBInfo.Guard.RLock()
	defer COMBInfo.Guard.RUnlock()
	for {
		if block, ok := (*blocks)[hash]; ok {
			block_chain = append(block_chain, hash)
			hash = block.Previous
			if _, ok := COMBInfo.Chain[hash]; ok {
				break
			}
		} else {
			break
		}
	}

	var progress float64 = (float64(len(block_chain)) / float64(length)) * 100.0
	combcore_set_status(fmt.Sprintf("Mining (%.2f%%)...", progress))

	//check if we actually found a known block
	if _, ok := COMBInfo.Chain[hash]; !ok {
		return nil //nope
	}

	//reverse chain so we mine old blocks first
	for i, j := 0, len(block_chain)-1; i < j; i, j = i+1, j-1 {
		block_chain[i], block_chain[j] = block_chain[j], block_chain[i]
	}

	return block_chain
}

func direct_load_trace(blocks *RawData, path string, target [32]byte, length uint64) (chain [][32]byte, err error) {
	var block_data []byte = make([]byte, 128*1024*1024) //blk files are max 128mb
	var block_files []string
	if block_files, err = filepath.Glob(path + "/blocks/blk*.dat"); err != nil {
		return nil, err
	}

	//reverse block files, so we trace backwards from new blocks (saves time)
	for i, j := 0, len(block_files)-1; i < j; i, j = i+1, j-1 {
		block_files[i], block_files[j] = block_files[j], block_files[i]
	}

	for b := range block_files {

		//read & process the block file and load it into blocks
		//we preallocate and pass in a 128mb buffer to read the block file into (block_data)
		direct_parse_block_file(block_data, blocks, block_files[b])

		log_status("direct", "processed %s", block_files[b])

		//now see if we have a valid chain loaded (from target to any block in history)
		chain = direct_trace_chain(blocks, target, length)

		if len(chain) != 0 {
			log_status("direct", "chain connected. finished")
			break //valid chain found
		}
	}
	return chain, nil
}

func direct_check_path(path string) (err error) {
	if path == "" {
		return fmt.Errorf("no path configured")
	}
	path = path + "/blocks"
	if _, err = os.Stat(path); err != nil {
		return err
	}
	var block_files []string
	if block_files, err = filepath.Glob(path + "/blk*.dat"); err != nil {
		return err
	}
	if len(block_files) == 0 {
		return fmt.Errorf("no block files found")
	}

	log_status("direct", "found %d block files", len(block_files))
	return nil
}
func direct_get_block_range(path string, target [32]byte, length uint64, out chan<- BlockData) (err error) {
	defer close(out)
	var blocks RawData = make(RawData)
	var chain [][32]byte

	//read the raw block data, tracing the blocks back from the target to a known block (probably a checkpoint)
	//processed blocks are stored IN MEMORY until a complete/valid chain is found (expect ~1gb of RAM usage)
	if chain, err = direct_load_trace(&blocks, path, target, length); err != nil {
		return err
	}

	//now that we have a known valid chain we can feed the blocks to neominer
	combcore_set_status("Storing...")
	for _, hash := range chain {
		out <- *blocks[hash]
	}
	return nil
}
