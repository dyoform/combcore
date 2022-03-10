package main

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"log"
	"math/rand"
	"sync"

	"libcomb"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

const DB_LEGACY_VERSION = 1
const DB_CURRENT_VERSION = 2

const DB_VERSION_KEY_LENGTH = 2
const DB_BLOCK_KEY_LENGTH = 8
const DB_COMMIT_KEY_LENGTH = 16

var db *leveldb.DB
var db_is_new bool
var db_mutex sync.Mutex

var DBInfo struct {
	InitialLoad     bool
	Version         uint16
	CorruptedBlocks map[uint64]struct{}
	Fingerprint     [32]byte
}

type BlockMetadata struct {
	Height      uint64
	Hash        [32]byte
	Previous    [32]byte
	Fingerprint [32]byte
}

type Block struct {
	Metadata BlockMetadata
	Commits  [][32]byte
}

func db_compute_block_fingerprint(commits [][32]byte) [32]byte {
	if len(commits) == 0 { //empty block has all zero fingerprint (saves compute)
		return [32]byte{}
	}
	var h hash.Hash = sha256.New()
	var fingerprint [32]byte
	for _, c := range commits {
		h.Write(c[:])
	}
	h.Sum(fingerprint[0:0])
	return fingerprint
}

func db_compute_db_fingerprint() [32]byte {
	var fingerprint [32]byte
	iter := db.NewIterator(nil, nil)
	var key []byte
	var value []byte
	var metadata BlockMetadata
	for iter.First(); iter.Valid(); iter.Next() {
		if len(iter.Key()) == DB_BLOCK_KEY_LENGTH {
			key = iter.Key()
			value = iter.Value()
			metadata = decode_block_metadata(key, value)
			fingerprint = xor_hex(fingerprint, metadata.Fingerprint)
		}
	}
	iter.Release()
	return fingerprint
}

func db_find_commits(commit [32]byte) (out []uint64) {
	var tmp [32]byte
	var height uint64 = 0
	iter := db.NewIterator(nil, nil)
	for iok := iter.First(); iok; iok = iter.Next() {
		if len(iter.Key()) == DB_BLOCK_KEY_LENGTH {
			height = binary.BigEndian.Uint64(iter.Key())
		}
		if len(iter.Key()) == DB_COMMIT_KEY_LENGTH {
			copy(tmp[:], iter.Value()[0:32])
			if tmp == commit {
				out = append(out, height)
			}
		}
	}
	iter.Release()
	return out
}

func db_open() (err error) {
	DBInfo.InitialLoad = true

	var lvldb *leveldb.DB
	var options opt.Options
	options.Compression = opt.NoCompression

	path := COMBInfo.Path

	//see if a db exists
	options.ErrorIfMissing = true
	lvldb, err = leveldb.OpenFile(path, &options)

	if err != nil {
		options.ErrorIfMissing = false
		//may not exist, try create one
		lvldb, err = leveldb.OpenFile(path, &options)
		if err == nil {
			db_is_new = true
		} else {
			//actually was some other error
			return err
		}
	}

	db_mutex.Lock()
	db = lvldb
	return nil
}

func db_close() {
	db.Close()
	db = nil
	db_mutex.Unlock()
}

func db_write(batch *leveldb.Batch) (err error) {
	critical.Lock()
	err = db.Write(batch, &opt.WriteOptions{
		Sync: true,
	})
	batch.Reset()
	critical.Unlock()
	return err
}

func decode_commit(data []byte) (commit [32]byte) {
	copy(commit[:], data[0:32])
	return commit
}

func decode_tag(data []byte) (tag libcomb.Tag) {
	tag.Height = binary.BigEndian.Uint64(data[0:8])
	tag.Order = binary.BigEndian.Uint32(data[8:12])
	return tag
}

func encode_tag(tag libcomb.Tag) (data [16]byte) {
	binary.BigEndian.PutUint64(data[0:8], tag.Height)
	binary.BigEndian.PutUint32(data[8:12], tag.Order)
	return data
}

func decode_block_metadata(key []byte, value []byte) (block BlockMetadata) {
	block.Height = binary.BigEndian.Uint64(key[0:8])
	copy(block.Hash[:], value[0:32])
	copy(block.Previous[:], value[32:64])
	copy(block.Fingerprint[:], value[64:96])
	return block
}

func encode_block_metadata(data BlockMetadata) (key [8]byte, value [96]byte) {
	binary.BigEndian.PutUint64(key[0:8], data.Height)
	copy(value[0:32], data.Hash[:])
	copy(value[32:64], data.Previous[:])
	copy(value[64:96], data.Fingerprint[:])
	return key, value
}

func db_inspect() {
	iter := db.NewIterator(nil, nil)

	var sizes map[uint16]uint64 = make(map[uint16]uint64)

	var total uint64
	var total_keys uint64
	for iok := iter.First(); iok; iok = iter.Next() {
		total += uint64(len(iter.Key())) + uint64(len(iter.Value()))
		total_keys++
		sizes[uint16(len(iter.Key()))] += 1
	}
	log.Println("Database:")
	log.Printf("\tSize: %0.2f mb\n", float64(total)/(1024*1024))
	log.Printf("\tKeys: %d\n", total_keys)
	for key, value := range sizes {
		log.Printf("\t\t%d: %d\n", key, value)
	}
	iter.Release()
}

func db_get_version() uint16 {
	var key [2]byte
	var version uint16 = 1
	if data, err := db.Get(key[:], nil); err == nil {
		version = binary.BigEndian.Uint16(data)
	}
	return version
}

func db_debug_remove_after(height uint64) {
	var start_key [8]byte = uint64_to_bytes(height)
	batch := new(leveldb.Batch)
	iter := db.NewIterator(nil, nil)
	iter.Seek(start_key[:])
	batch.Delete(iter.Key())
	for iter.Next() {
		batch.Delete(iter.Key())
	}
	iter.Release()
	db_write(batch)
}

func db_debug_corrupt_after(height uint64) {
	batch := new(leveldb.Batch)
	for {
		if rand.Float64() < 0.5 {
			var key [8]byte = uint64_to_bytes(height)
			if value, err := db.Get(key[:], nil); err == nil {
				value[0] = 0
				batch.Put(key[:], value)
			} else {
				break
			}
		}
		height++
	}
	db_write(batch)
}

func db_store_block(batch *leveldb.Batch, block *Block) (err error) {
	var current_tag libcomb.Tag
	current_tag.Height = block.Metadata.Height
	current_tag.Order = 0

	for _, commit := range block.Commits {
		tag_data := encode_tag(current_tag)
		batch.Put(tag_data[:], commit[:])
		current_tag.Order++
	}

	key, data := encode_block_metadata(block.Metadata)
	batch.Put(key[:], data[:])
	return err
}

func db_remove_block(batch *leveldb.Batch, height uint64) (err error) {
	var prefix [8]byte
	binary.BigEndian.PutUint64(prefix[:], height)
	iter := db.NewIterator(util.BytesPrefix(prefix[:]), nil)
	for iter.Next() {
		batch.Delete(iter.Key())
	}
	iter.Release()
	if err = iter.Error(); err != nil {
		return err
	}
	return nil
}

func db_remove_blocks_after(height uint64) (err error) {
	var batch *leveldb.Batch = new(leveldb.Batch)
	var prefix [8]byte
	binary.BigEndian.PutUint64(prefix[:], height)
	iter := db.NewIterator(nil, nil)
	iter.Seek(prefix[:])
	for iter.Next() {
		batch.Delete(iter.Key())
	}
	iter.Release()
	if err = iter.Error(); err != nil {
		return err
	}
	db_write(batch)
	return nil
}

func db_process_block(batch *leveldb.Batch, block Block) (err error) {
	if err = db_remove_block(batch, block.Metadata.Height); err != nil {
		return err
	}

	if err = db_store_block(batch, &block); err != nil {
		return err
	}
	return nil
}

func db_get_block_metadata_by_hash(hash [32]byte) (metadata BlockMetadata) {
	iter := db.NewIterator(nil, nil)
	var key []byte
	var value []byte
	var m BlockMetadata
	//iterate in reverse, it will be faster most of the time

	for iter.Last(); iter.Valid(); iter.Prev() {
		if len(iter.Key()) == DB_BLOCK_KEY_LENGTH {
			key = iter.Key()
			value = iter.Value()
			m = decode_block_metadata(key, value)
			if m.Hash == hash {
				metadata = m
				break
			}
		}
	}
	iter.Release()
	return metadata
}

func db_get_block_metadata_by_height(height uint64) (metadata BlockMetadata) {
	var seek_key [8]byte
	binary.BigEndian.PutUint64(seek_key[0:8], height)

	iter := db.NewIterator(nil, nil)
	var key []byte
	var value []byte

	if ok := iter.Seek(seek_key[:]); ok {
		key = iter.Key()
		value = iter.Value()
		metadata = decode_block_metadata(key, value)
	}

	iter.Release()

	return metadata
}

func db_get_block_by_height(height uint64) (block BlockData) {
	var seek_key [8]byte
	binary.BigEndian.PutUint64(seek_key[0:8], height)

	iter := db.NewIterator(nil, nil)
	var key []byte
	var value []byte

	if ok := iter.Seek(seek_key[:]); ok {
		key = iter.Key()
		value = iter.Value()
		metadata := decode_block_metadata(key, value)
		if metadata.Height == height {

			// Set hash and prev
			block.Hash = metadata.Hash
			block.Previous = metadata.Previous
		}

		for iter.Next() {
			if len(iter.Key()) == DB_COMMIT_KEY_LENGTH {
				// Found commit, add to block
				block.Commits = append(block.Commits, decode_commit(value))
			} else {
				// Found next metadata, stop
				break
			}
		}
	}

	iter.Release()
	return block
}

func db_load_blocks(start, end uint64, out chan<- Block) {
	var iter iterator.Iterator
	var start_bytes [8]byte
	var end_bytes [8]byte
	var key []byte
	var value []byte
	var block Block
	var commit [32]byte
	var is_empty bool = true

	defer close(out)

	binary.BigEndian.PutUint64(start_bytes[:], start)
	binary.BigEndian.PutUint64(end_bytes[:], end+1)

	iter = db.NewIterator(&util.Range{Start: start_bytes[:], Limit: end_bytes[:]}, nil)

	for iter.Next() {
		key = iter.Key()
		value = iter.Value()
		switch len(key) {
		case DB_BLOCK_KEY_LENGTH:
			out <- block
			is_empty = false
			block.Metadata = decode_block_metadata(key, value)
			block.Commits = nil
		case DB_COMMIT_KEY_LENGTH:
			commit = decode_commit(value)
			block.Commits = append(block.Commits, commit)
		}
	}
	if !is_empty {
		out <- block
	}
	iter.Release()
}

func db_load() {
	var blocks chan Block = make(chan Block)
	var count uint64
	var wait sync.Mutex
	wait.Lock()
	go func() {
		for block := range blocks {
			var fingerprint [32]byte = db_compute_block_fingerprint(block.Commits)
			if block.Metadata.Fingerprint != fingerprint {
				//recovery not implemented yet
				log.Panicf("(db) fingerprint mismatch on block %d (%X != %X)\n", block.Metadata.Height, block.Metadata.Fingerprint, fingerprint)
			}
			combcore_process_block(block)
			count++
		}
		wait.Unlock()
	}()
	db_load_blocks(0, (^uint64(0))-1, blocks)
	wait.Lock()

	log.Printf("(db) loaded %d blocks\n", count)
}

func db_new() {
	batch := new(leveldb.Batch)
	var key [2]byte
	var value [2]byte
	binary.BigEndian.PutUint16(value[:], DB_CURRENT_VERSION)
	batch.Put(key[:], value[:])
	db_write(batch)
	DBInfo.Version = DB_CURRENT_VERSION
}

func db_start() {
	if db_is_new {
		log.Printf("(db) new database created (version %d)\n", DB_CURRENT_VERSION)
		db_new()
		return
	}

	log.Printf("(db) started. loading...")

	DBInfo.Version = db_get_version()
	if DBInfo.Version != DB_CURRENT_VERSION {
		log.Panicln("(db) cannot load legacy db")
	}

	db_load()
	DBInfo.InitialLoad = false
}
