package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"libcomb"
)

type Key struct {
	Public  string
	Private [21]string
	Balance uint64
	Active  bool
}

type Stack struct {
	Destination string
	Sum         uint64
	Change      string
	Address     string
	Balance     uint64
	Active      bool
}

type UnsignedTransaction struct {
	Source      string
	Destination string
	ID          string
}

type Transaction struct {
	Source      string
	Destination string
	Signature   [21]string
	Active      bool
	ID          string
}

type Decider struct {
	Private [2]string
	Tips    [2]string
	ID      string
}

type MerkleSegment struct {
	Tips      [2]string
	Signature [2]string
	Branches  [16]string
	Leaf      string
	Next      string

	Balance uint64
	Active  bool
	ID      string
	Root    string
}

type UnsignedMerkleSegment struct {
	Tips [2]string
	Next string
	Root string

	Balance uint64
	ID      string
}

func wallet_load_stack(data []byte) (address [32]byte, err error) {
	var stack libcomb.Stack
	if len(data) != 32+32+8 {
		return address, errors.New("stack data malformed")
	}
	copy(stack.Change[:], data[0:32])
	copy(stack.Destination[:], data[32:64])
	stack.Sum = binary.BigEndian.Uint64(data[64:])
	return libcomb.LoadStack(stack), nil
}

func wallet_load_key(data []byte) (address [32]byte, err error) {
	var key libcomb.Key
	if len(data) != 21*32 {
		return address, errors.New("key data malformed")
	}
	for i := range key.Private {
		copy(key.Private[i][:], data[i*32:(i+1)*32])
	}
	return libcomb.LoadKey(key), nil
}

func wallet_load_transaction(data []byte) (address [32]byte, err error) {
	var tx libcomb.Transaction
	if len(data) != 23*32 {
		return address, errors.New("tx data malformed")
	}

	copy(tx.Source[:], data[0:32])
	copy(tx.Destination[:], data[32:64])

	for i := range tx.Signature {
		copy(tx.Signature[i][:], data[i*32+64:(i+1)*32+64])
	}
	if address, err = libcomb.LoadTransaction(tx); err != nil {
		return address, err
	}
	return address, nil
}

func wallet_load_merkle_segment(data []byte) (address [32]byte, err error) {
	var m libcomb.MerkleSegment
	if len(data) != 22*32 {
		return address, errors.New("merkle segment data malformed")
	}
	copy(m.Tips[0][:], data[0:32])
	copy(m.Tips[1][:], data[32:64])
	copy(m.Signature[0][:], data[64:96])
	copy(m.Signature[1][:], data[96:128])

	data = data[128:]
	for i := range m.Branches {
		copy(m.Branches[i][:], data[i*32:(i+1)*32])
	}
	data = data[16*32:]

	copy(m.Leaf[:], data[0:32])
	copy(m.Next[:], data[32:64])

	if address, err = libcomb.LoadMerkleSegment(m); err != nil {
		return address, err
	}

	return address, nil
}

func wallet_load_unsigned_merkle_segment(data []byte) (address [32]byte, err error) {
	var m libcomb.UnsignedMerkleSegment
	if len(data) != 4*32 {
		return address, errors.New("unsigned merkle segment data malformed")
	}
	copy(m.Tips[0][:], data[0:32])
	copy(m.Tips[1][:], data[32:64])
	copy(m.Next[:], data[64:96])
	copy(m.Root[:], data[96:128])

	if address, err = libcomb.LoadUnsignedMerkleSegment(m); err != nil {
		return address, err
	}

	return address, nil
}

func wallet_load_decider(data []byte) (short [32]byte, err error) {
	var d libcomb.Decider

	if len(data) == 3*32 { //old format {next, private_1, private_2}
		copy(d.Private[0][:], data[32:64])
		copy(d.Private[1][:], data[64:96])
		return libcomb.LoadDecider(d), nil
	}

	if len(data) == 2*32 { //new format {private_1, private_2}
		copy(d.Private[0][:], data[0:32])
		copy(d.Private[1][:], data[32:64])
		return libcomb.LoadDecider(d), nil
	}

	return short, errors.New("decider data malformed")
}

func wallet_load_construct(construct string) (address [32]byte, err error) {
	var data []byte
	if strings.HasPrefix(construct, COMBInfo.Prefix["stack"]) {
		construct = strings.TrimPrefix(construct, COMBInfo.Prefix["stack"])
		data = hex2byte([]byte(construct))
		address, err = wallet_load_stack(data)
	}
	if strings.HasPrefix(construct, COMBInfo.Prefix["tx"]) {
		construct = strings.TrimPrefix(construct, COMBInfo.Prefix["tx"])
		data = hex2byte([]byte(construct))
		address, err = wallet_load_transaction(data)
	}

	if strings.HasPrefix(construct, COMBInfo.Prefix["key"]) {
		construct = strings.TrimPrefix(construct, COMBInfo.Prefix["key"])
		data = hex2byte([]byte(construct))
		address, err = wallet_load_key(data)
	}

	if strings.HasPrefix(construct, COMBInfo.Prefix["merkle"]) {
		construct = strings.TrimPrefix(construct, COMBInfo.Prefix["merkle"])
		data = hex2byte([]byte(construct))
		address, err = wallet_load_merkle_segment(data)
	}

	if strings.HasPrefix(construct, COMBInfo.Prefix["decider"]) {
		construct = strings.TrimPrefix(construct, COMBInfo.Prefix["decider"])
		data = hex2byte([]byte(construct))
		address, err = wallet_load_decider(data)
	}

	if strings.HasPrefix(construct, COMBInfo.Prefix["unsigned_merkle"]) {
		construct = strings.TrimPrefix(construct, COMBInfo.Prefix["unsigned_merkle"])
		data = hex2byte([]byte(construct))
		address, err = wallet_load_unsigned_merkle_segment(data)
	}
	return address, err
}

func wallet_load(data string) (err error) {
	combcore_set_status("Loading Wallet...")
	combcore_lock_status()
	defer combcore_set_status("Idle")
	defer combcore_unlock_status()

	var lines []string = strings.Split(data, "\n")
	var address [32]byte
	for _, line := range lines {
		if line == "" {
			continue
		}
		if address, err = wallet_load_construct(line); err != nil {
			LogError("import", "load construct error (%s)", err.Error())
			return err
		} else {
			LogStatus("import", "loaded construct (%X)", address)
		}
	}

	return nil
}

func wallet_parse_key(w Key) (lw libcomb.Key, err error) {
	if lw.Public, err = parse_hex(w.Public); err != nil {
		return libcomb.Key{}, err
	}
	for i := range w.Private {
		if lw.Private[i], err = parse_hex(w.Private[i]); err != nil {
			return libcomb.Key{}, err
		}
	}
	return lw, err
}

func wallet_parse_stack(s Stack) (ls libcomb.Stack, err error) {
	if ls.Destination, err = parse_hex(s.Destination); err != nil {
		return libcomb.Stack{}, err
	}
	ls.Sum = s.Sum
	if ls.Change, err = parse_hex(s.Change); err != nil {
		return libcomb.Stack{}, err
	}
	return ls, err
}

func wallet_parse_unsigned_transaction(tx UnsignedTransaction) (ltx libcomb.Transaction, err error) {
	if ltx.Destination, err = parse_hex(tx.Destination); err != nil {
		return libcomb.Transaction{}, err
	}
	if ltx.Source, err = parse_hex(tx.Source); err != nil {
		return libcomb.Transaction{}, err
	}
	var tmptx libcomb.Transaction
	tmptx.Source = ltx.Source
	tmptx.Destination = ltx.Destination
	return ltx, err
}

func wallet_parse_transaction(tx Transaction) (ltx libcomb.Transaction, err error) {
	if ltx.Destination, err = parse_hex(tx.Destination); err != nil {
		return libcomb.Transaction{}, err
	}
	if ltx.Source, err = parse_hex(tx.Source); err != nil {
		return libcomb.Transaction{}, err
	}
	for i := range tx.Signature {
		if ltx.Signature[i], err = parse_hex(tx.Signature[i]); err != nil {
			return libcomb.Transaction{}, err
		}
	}
	return ltx, err
}

func wallet_parse_decider(d Decider) (ld libcomb.Decider, err error) {
	if ld.Private[0], err = parse_hex(d.Private[0]); err != nil {
		return libcomb.Decider{}, err
	}
	if ld.Private[1], err = parse_hex(d.Private[1]); err != nil {
		return libcomb.Decider{}, err
	}

	ld = libcomb.RecoverDecider(ld)

	return ld, err
}

func wallet_parse_merkle_segment(m MerkleSegment) (lm libcomb.MerkleSegment, err error) {
	if lm.Tips[0], err = parse_hex(m.Tips[0]); err != nil {
		return libcomb.MerkleSegment{}, err
	}
	if lm.Tips[1], err = parse_hex(m.Tips[1]); err != nil {
		return libcomb.MerkleSegment{}, err
	}
	if lm.Signature[0], err = parse_hex(m.Signature[0]); err != nil {
		return libcomb.MerkleSegment{}, err
	}
	if lm.Signature[1], err = parse_hex(m.Signature[1]); err != nil {
		return libcomb.MerkleSegment{}, err
	}
	if lm.Leaf, err = parse_hex(m.Leaf); err != nil {
		return libcomb.MerkleSegment{}, err
	}
	if lm.Next, err = parse_hex(m.Next); err != nil {
		return libcomb.MerkleSegment{}, err
	}
	for i := range m.Branches {
		if lm.Branches[i], err = parse_hex(m.Branches[i]); err != nil {
			return libcomb.MerkleSegment{}, err
		}
	}

	return lm, err
}

func wallet_parse_unsigned_merkle_segment(c UnsignedMerkleSegment) (lc libcomb.UnsignedMerkleSegment, err error) {
	if lc.Tips[0], err = parse_hex(c.Tips[0]); err != nil {
		return libcomb.UnsignedMerkleSegment{}, err
	}
	if lc.Tips[1], err = parse_hex(c.Tips[1]); err != nil {
		return libcomb.UnsignedMerkleSegment{}, err
	}
	if lc.Next, err = parse_hex(c.Next); err != nil {
		return libcomb.UnsignedMerkleSegment{}, err
	}
	if lc.Root, err = parse_hex(c.Root); err != nil {
		return libcomb.UnsignedMerkleSegment{}, err
	}

	return lc, err
}

func wallet_stringify_key(w libcomb.Key) (sw Key) {
	sw.Public = stringify_hex(w.Public)
	for i := range w.Private {
		sw.Private[i] = stringify_hex(w.Private[i])
	}
	sw.Balance = libcomb.GetBalance(w.Public)
	sw.Active = w.Active()
	return sw
}

func wallet_stringify_stack(s libcomb.Stack) (ss Stack) {
	ss.Change = stringify_hex(s.Change)
	ss.Destination = stringify_hex(s.Destination)
	ss.Sum = s.Sum
	ss.Address = stringify_hex(s.ID())
	ss.Active = s.Active()
	ss.Balance = libcomb.GetBalance(s.ID())
	return ss
}

func wallet_stringify_transaction(tx libcomb.Transaction) (stx Transaction) {
	stx.Source = stringify_hex(tx.Source)
	stx.Destination = stringify_hex(tx.Destination)
	for i := range tx.Signature {
		stx.Signature[i] = stringify_hex(tx.Signature[i])
	}
	stx.ID = stringify_hex(tx.ID())
	stx.Active = tx.Active()
	return stx
}

func wallet_stringify_decider(d libcomb.Decider) (sd Decider) {
	sd.Private[0] = stringify_hex(d.Private[0])
	sd.Private[1] = stringify_hex(d.Private[1])
	sd.Tips[0] = stringify_hex(d.Tips[0])
	sd.Tips[1] = stringify_hex(d.Tips[1])
	sd.ID = stringify_hex(d.ID())
	return sd
}

func wallet_stringify_unsigned_merkle_segment(c libcomb.UnsignedMerkleSegment) (sc UnsignedMerkleSegment) {
	sc.Next = stringify_hex(c.Next)
	sc.Root = stringify_hex(c.Root)
	sc.Tips[0] = stringify_hex(c.Tips[0])
	sc.Tips[1] = stringify_hex(c.Tips[1])
	sc.ID = stringify_hex(c.ID())
	sc.Balance = libcomb.GetBalance(c.ID())
	return sc
}

func wallet_stringify_merkle_segment(m libcomb.MerkleSegment) (sm MerkleSegment) {
	sm.Tips[0] = stringify_hex(m.Tips[0])
	sm.Tips[1] = stringify_hex(m.Tips[1])

	sm.Signature[0] = stringify_hex(m.Signature[0])
	sm.Signature[1] = stringify_hex(m.Signature[1])

	sm.Leaf = stringify_hex(m.Leaf)
	sm.Next = stringify_hex(m.Next)

	for i := range m.Branches {
		sm.Branches[i] = stringify_hex(m.Branches[i])
	}

	sm.ID = stringify_hex(m.ID())
	sm.Active = m.Active()
	sm.Balance = libcomb.GetBalance(m.ID())
	sm.Root = stringify_hex(m.Root)
	return sm
}

type StringWallet struct {
	Keys            []Key
	Stacks          []Stack
	TXs             []Transaction
	Deciders        []Decider
	Merkles         []MerkleSegment
	UnsignedMerkles []UnsignedMerkleSegment
}

func wallet_stringify() StringWallet {
	var w StringWallet
	for _, k := range libcomb.GetKeys() {
		w.Keys = append(w.Keys, wallet_stringify_key(k))
	}
	for _, s := range libcomb.GetStacks() {
		w.Stacks = append(w.Stacks, wallet_stringify_stack(s))
	}
	for _, tx := range libcomb.GetTransactions() {
		w.TXs = append(w.TXs, wallet_stringify_transaction(tx))
	}
	for _, d := range libcomb.GetDeciders() {
		w.Deciders = append(w.Deciders, wallet_stringify_decider(d))
	}
	for _, m := range libcomb.GetMerkleSegments() {
		w.Merkles = append(w.Merkles, wallet_stringify_merkle_segment(m))
	}
	for _, u := range libcomb.GetUnsignedMerkleSegments() {
		w.UnsignedMerkles = append(w.UnsignedMerkles, wallet_stringify_unsigned_merkle_segment(u))
	}
	return w
}

func wallet_export_key(w libcomb.Key) (out string) {
	out = COMBInfo.Prefix["key"]
	for _, k := range w.Private {
		out += stringify_hex(k)
	}
	return out
}

func wallet_export_stack(s libcomb.Stack) (out string) {
	var sum [8]byte = uint64_to_bytes(s.Sum)
	out = fmt.Sprintf("%s%X%X%X", COMBInfo.Prefix["stack"], s.Change, s.Destination, sum)
	return out
}

func wallet_export_transaction(tx libcomb.Transaction) (out string) {
	out = fmt.Sprintf("%s%X%X", COMBInfo.Prefix["tx"], tx.Source, tx.Destination)
	for _, k := range tx.Signature {
		out += stringify_hex(k)
	}
	return out
}

func wallet_export_decider(d libcomb.Decider, next [32]byte) (out string) {
	out = fmt.Sprintf("%s%X%X%X", COMBInfo.Prefix["decider"], next, d.Private[0], d.Private[1])
	return out
}

func wallet_export_merkle_segment(m libcomb.MerkleSegment) (out string) {
	out = fmt.Sprintf("%s%X%X%X%X", COMBInfo.Prefix["merkle"], m.Tips[0], m.Tips[1], m.Signature[0], m.Signature[1])
	for _, b := range m.Branches {
		out += stringify_hex(b)
	}
	out += fmt.Sprintf("%X%X", m.Leaf, m.Next)
	return out
}

func wallet_export_unsigned_merkle_segment(m libcomb.UnsignedMerkleSegment) (out string) {
	out = fmt.Sprintf("%s%X%X%X%X", COMBInfo.Prefix["unsigned_merkle"], m.Tips[0], m.Tips[1], m.Next, m.Root)
	return out
}

func wallet_export() (out string) {
	var empty [32]byte
	for _, k := range libcomb.GetKeys() {
		out += wallet_export_key(k) + "\n"
	}
	for _, s := range libcomb.GetStacks() {
		out += wallet_export_stack(s) + "\n"
	}
	for _, tx := range libcomb.GetTransactions() {
		out += wallet_export_transaction(tx) + "\n"
	}
	for _, d := range libcomb.GetDeciders() {
		out += wallet_export_decider(d, empty) + "\n"
	}
	for _, m := range libcomb.GetMerkleSegments() {
		out += wallet_export_merkle_segment(m) + "\n"
	}
	for _, m := range libcomb.GetUnsignedMerkleSegments() {
		out += wallet_export_unsigned_merkle_segment(m) + "\n"
	}
	return out
}

func wallet_export_history(history map[[32]byte]struct{}) (out string) {
	var empty [32]byte
	for _, k := range libcomb.GetKeys() {
		if _, ok := history[k.ID()]; ok {
			out += wallet_export_key(k) + "\n"
		}
	}
	for _, s := range libcomb.GetStacks() {
		if _, ok := history[s.ID()]; ok {
			out += wallet_export_stack(s) + "\n"
		}
	}
	for _, tx := range libcomb.GetTransactions() {
		if _, ok := history[tx.ID()]; ok {
			out += wallet_export_transaction(tx) + "\n"
		}
	}
	for _, d := range libcomb.GetDeciders() {
		if _, ok := history[d.ID()]; ok {
			out += wallet_export_decider(d, empty) + "\n"
		}
	}
	for _, m := range libcomb.GetMerkleSegments() {
		if _, ok := history[m.ID()]; ok {
			out += wallet_export_merkle_segment(m) + "\n"
		}
	}
	for _, m := range libcomb.GetUnsignedMerkleSegments() {
		if _, ok := history[m.ID()]; ok {
			out += wallet_export_unsigned_merkle_segment(m) + "\n"
		}
	}
	return out
}
