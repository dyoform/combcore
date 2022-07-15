package main

import (
	"fmt"

	"libcomb"
)

type Control struct{}

func (c *Control) LoadTransaction(args *Transaction, reply *string) (err error) {
	var tx libcomb.Transaction

	if tx, err = wallet_parse_transaction(*args); err != nil {
		return err
	}

	var id [32]byte
	if id, err = libcomb.LoadTransaction(tx); err != nil {
		return err
	}

	*reply = stringify_hex(id)
	return nil
}
func (c *Control) LoadKey(args *Key, reply *string) (err error) {
	var w libcomb.Key
	if w, err = wallet_parse_key(*args); err != nil {
		return err
	}
	var address [32]byte = libcomb.LoadKey(w)
	*reply = stringify_hex(address)
	return nil
}
func (c *Control) LoadStack(args *Stack, reply *string) (err error) {
	var s libcomb.Stack
	if s, err = wallet_parse_stack(*args); err != nil {
		return err
	}
	var address [32]byte = libcomb.LoadStack(s)
	*reply = stringify_hex(address)
	return nil
}
func (c *Control) LoadDecider(args *Decider, reply *string) (err error) {
	var d libcomb.Decider
	if d, err = wallet_parse_decider(*args); err != nil {
		return err
	}
	var id [32]byte = libcomb.LoadDecider(d)
	*reply = stringify_hex(id)
	return nil
}
func (c *Control) LoadMerkleSegment(args *MerkleSegment, reply *string) (err error) {
	var m libcomb.MerkleSegment
	if m, err = wallet_parse_merkle_segment(*args); err != nil {
		return err
	}

	var id [32]byte

	if id, err = libcomb.LoadMerkleSegment(m); err != nil {
		return err
	}

	*reply = stringify_hex(id)
	return nil
}

func (c *Control) GenerateKey(args *interface{}, reply *Key) error {
	key, _ := libcomb.NewKey()
	*reply = wallet_stringify_key(key)
	return nil
}

func (c *Control) ConstructStack(args *Stack, reply *Stack) (err error) {
	*reply = *args
	var s libcomb.Stack
	if s, err = wallet_parse_stack(*args); err != nil {
		return err
	}
	(*reply).Address = stringify_hex(s.ID())
	return err
}

func (c *Control) GenerateDecider(args *interface{}, reply *Decider) error {
	decider, _ := libcomb.NewDecider()
	*reply = wallet_stringify_decider(decider)
	return nil
}

func (c *Control) ConstructTransaction(args *UnsignedTransaction, result *Transaction) (err error) {
	var tx libcomb.Transaction
	if tx, err = wallet_parse_unsigned_transaction(*args); err != nil {
		return err
	}

	if err = libcomb.SignTransaction(&tx); err != nil {
		return err
	}

	*result = wallet_stringify_transaction(tx)
	return nil
}

type SignDeciderArgs struct {
	ID          string
	Destination int
}

func (c *Control) SignDecider(args *SignDeciderArgs, result *[2]string) (err error) {
	var d libcomb.Decider
	var id [32]byte
	if id, err = parse_hex(args.ID); err != nil {
		return err
	}
	if d, err = libcomb.LookupDecider(id); err != nil {
		return err
	}

	var s [2][32]byte
	if s, err = libcomb.SignDecider(d, uint16(args.Destination)); err != nil {
		return err
	}
	*result = [2]string{stringify_hex(s[0]), stringify_hex(s[1])}
	return nil
}

type ConstructUnsignedMerkleSegmentArgs struct {
	Tips [2]string
	Next string
	Root string
}

func (c *Control) ConstructUnsignedMerkleSegment(args *ConstructUnsignedMerkleSegmentArgs, result *UnsignedMerkleSegment) (err error) {
	var m libcomb.UnsignedMerkleSegment
	if m.Tips[0], err = parse_hex(args.Tips[0]); err != nil {
		return err
	}
	if m.Tips[1], err = parse_hex(args.Tips[1]); err != nil {
		return err
	}
	if m.Root, err = parse_hex(args.Root); err != nil {
		return err
	}
	if m.Next, err = parse_hex(args.Next); err != nil {
		return err
	}

	*result = wallet_stringify_unsigned_merkle_segment(m)
	return nil
}

func (c *Control) LoadUnsignedMerkleSegment(args *UnsignedMerkleSegment, reply *string) (err error) {
	var m libcomb.UnsignedMerkleSegment
	if m, err = wallet_parse_unsigned_merkle_segment(*args); err != nil {
		return err
	}

	var id [32]byte

	if id, err = libcomb.LoadUnsignedMerkleSegment(m); err != nil {
		return err
	}

	*reply = stringify_hex(id)
	return nil
}

func (c *Control) ComputeRoot(args *[]string, result *string) (err error) {
	var tree [65536][32]byte
	var root [32]byte

	if len(*args) > 65536 {
		return fmt.Errorf("tree has too many leaves")
	}

	for i, leaf := range *args {
		if h, err := parse_hex(leaf); err != nil {
			return err
		} else {
			tree[i] = h
		}
	}

	root, _, _ = libcomb.ComputeProof(tree, 0)
	*result = stringify_hex(root)
	return nil
}

type ComputeProofArgs struct {
	Tree        []string
	Destination int
}

type ComputeProofResult struct {
	Root     string
	Leaf     string
	Branches string
}

func (c *Control) ComputeProof(args *ComputeProofArgs, result *ComputeProofResult) (err error) {
	var tree [65536][32]byte
	var root [32]byte
	var branches [16][32]byte
	var leaf [32]byte

	if len(args.Tree) > 65536 {
		return fmt.Errorf("tree has too many leaves")
	}

	if args.Destination < 0 || args.Destination > 65535 {
		return fmt.Errorf("destination out of range")
	}

	for i, leaf := range args.Tree {
		if h, err := parse_hex(leaf); err != nil {
			return err
		} else {
			tree[i] = h
		}
	}

	root, branches, leaf = libcomb.ComputeProof(tree, uint16(args.Destination))

	result.Root = stringify_hex(root)
	result.Leaf = stringify_hex(leaf)
	result.Branches = ""
	for _, b := range branches {
		result.Branches += stringify_hex(b)
	}

	return nil
}

type DecideMerkleSegmentArgs struct {
	Address   string
	Signature [2]string
	Branches  string
	Leaf      string
}

func (c *Control) DecideMerkleSegment(args *DecideMerkleSegmentArgs, result *MerkleSegment) (err error) {
	var u libcomb.UnsignedMerkleSegment
	var m libcomb.MerkleSegment

	var address [32]byte
	if address, err = parse_hex(args.Address); err != nil {
		return err
	}

	if m.Signature[0], err = parse_hex(args.Signature[0]); err != nil {
		return err
	}
	if m.Signature[1], err = parse_hex(args.Signature[1]); err != nil {
		return err
	}

	if len(args.Branches) != 16*32*2 {
		return fmt.Errorf("branches need to be 512 bytes")
	}

	for i := range m.Branches {
		if m.Branches[i], err = parse_hex(args.Branches[i*64 : (i+1)*64]); err != nil {
			return err
		}
	}
	if m.Leaf, err = parse_hex(args.Leaf); err != nil {
		return err
	}

	if u, err = libcomb.LookupUnsignedMerkleSegment(address); err != nil {
		return err
	}

	m.Tips = u.Tips
	m.Next = u.Next

	if err = libcomb.RecoverMerkleSegment(&m); err != nil {
		return err
	}

	if m.ID() != u.ID() {
		return fmt.Errorf("address mismatch. branches, leaf or signature is invalid")
	}

	*result = wallet_stringify_merkle_segment(m)
	return nil
}

func (c *Control) GetAddressBalance(args *string, reply *uint64) (err error) {
	var address [32]byte
	if address, err = parse_hex(*args); err != nil {
		return err
	}
	*reply = libcomb.GetBalance(address)
	return nil
}

func (c *Control) CommitAddress(args *string, reply *string) (err error) {
	var address [32]byte
	if address, err = parse_hex(*args); err != nil {
		return err
	}
	address = libcomb.Commit(address)
	*reply = stringify_hex(address)
	return nil
}

func (c *Control) CommitAddresses(args *[]string, reply *[]string) (err error) {
	var address [32]byte

	for _, a := range *args {
		if address, err = parse_hex(a); err != nil {
			return err
		}
		address = libcomb.Commit(address)
		*reply = append(*reply, stringify_hex(address))
	}
	return nil
}

func (c *Control) CheckAddresses(args *[]string, reply *[]string) (err error) {
	//takes list of addresses and returns the addresses that are not committed
	var address [32]byte

	for _, a := range *args {
		if address, err = parse_hex(a); err != nil {
			return err
		}
		address = libcomb.Commit(address)
		if !libcomb.HaveCommit(address) {
			*reply = append(*reply, a)
		}
	}
	return nil
}

func (c *Control) GetCOMBBase(args *int, reply *string) (err error) {
	var height uint64 = uint64(*args)
	var combbase [32]byte
	if combbase, err = libcomb.GetCOMBBase(height); err != nil {
		return err
	}
	*reply = stringify_hex(combbase)
	return nil
}

func (c *Control) GetTag(args *string, reply *libcomb.Tag) (err error) {
	var commit [32]byte
	if commit, err = parse_hex(*args); err != nil {
		return err
	}
	if *reply, err = libcomb.GetCommitTag(commit); err != nil {
		return err
	}
	return nil
}

func (c *Control) GetCoinHistory(args *string, reply *string) (err error) {
	var address [32]byte
	if address, err = parse_hex(*args); err != nil {
		return err
	}
	*reply = wallet_export_history(libcomb.GetCoinHistory(address))
	return nil
}

func (c *Control) LoadWallet(args *string, reply *struct{}) (err error) {
	err = wallet_load(*args)
	return err
}

func (c *Control) SaveWallet(args *struct{}, reply *string) (err error) {
	*reply = wallet_export()
	return err
}

func (c *Control) GetWallet(args *struct{}, reply *StringWallet) (err error) {
	*reply = wallet_stringify()
	return nil
}

type BlockReply struct {
	Hash   string
	Height int
}

func (c *Control) GetBlockByHeight(args *int, reply *BlockReply) (err error) {
	var height uint64 = uint64(*args)
	var metadata BlockMetadata = db_get_block_metadata_by_height(height)
	reply.Hash = stringify_hex(metadata.Hash)
	reply.Height = int(metadata.Height)
	return nil
}

type StatusReply struct {
	COMBHeight     uint64
	BTCHeight      uint64
	BTCKnownHeight uint64
	Commits        uint64
	Status         string
	Network        string
}

func (c *Control) GetStatus(args *struct{}, reply *StatusReply) (err error) {
	COMBInfo.Guard.RLock()
	defer COMBInfo.Guard.RUnlock()
	BTCInfo.Guard.RLock()
	defer BTCInfo.Guard.RUnlock()

	reply.COMBHeight = COMBInfo.Height
	reply.BTCHeight = BTCInfo.Chain.Height
	reply.BTCKnownHeight = BTCInfo.Chain.KnownHeight
	reply.Commits = libcomb.GetCommitCount()
	reply.Status = GUIInfo.Status
	reply.Network = COMBInfo.Network
	return nil
}

func (c *Control) GetFingerprint(args *struct{}, reply *string) (err error) {
	*reply = stringify_hex(db_compute_db_fingerprint())
	return nil
}

type PushBlockArgs struct {
	Hash     string
	Previous string
	Commits  []string
}

func (c *Control) PushBlocks(args *[]PushBlockArgs, reply *struct{}) (err error) {
	//using this function while mining or loading will cause a panic!

	if DBInfo.InitialLoad {
		return fmt.Errorf("cannot push during initial load")
	}

	for _, b := range *args {
		var blk BlockData
		if blk.Hash, err = parse_hex(b.Hash); err != nil {
			return err
		}
		if blk.Previous, err = parse_hex(b.Previous); err != nil {
			return err
		}

		blk.Commits = make([][32]byte, 0)
		for _, c := range b.Commits {
			if commit, err := parse_hex(c); err == nil {
				blk.Commits = append(blk.Commits, commit)
			} else {
				return err
			}
		}
		ingest_process_block(blk)
	}
	ingest_write()

	return nil
}

func (c *Control) GetChainTip(args *struct{}, reply *string) (err error) {
	COMBInfo.Guard.RLock()
	*reply = stringify_hex(COMBInfo.Hash)
	COMBInfo.Guard.RUnlock()
	return nil
}
