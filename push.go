package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var PushInfo struct {
	IP      string
	Port    uint16
	Enabled bool
}

func push_init() {
	PushInfo.Enabled = *push_ip != ""
	if !PushInfo.Enabled {
		return
	}
	PushInfo.IP = *push_ip
	PushInfo.Port = uint16(*push_port)

	log_status("push", "enabled. pushing to %s:%d", PushInfo.IP, PushInfo.Port)
}

func push_sync() {
	if !PushInfo.Enabled {
		return
	}

	var err error
	var ok bool
	var client *http.Client = &http.Client{}
	client.Timeout = time.Second

	var tip [32]byte
	if tip, err = push_get_chain_tip(client); err != nil {
		log_error("push", "failed to get chain tip (%s)", err.Error())
		return
	}

	COMBInfo.Guard.RLock()
	if tip == COMBInfo.Hash {
		COMBInfo.Guard.RUnlock()
		return
	}

	if _, ok = COMBInfo.Chain[tip]; !ok {
		//resolve this by finding the highest common block then instructing the client to reorg
		log_error("push", "client has an unknown chain")
		COMBInfo.Guard.RUnlock()
		return
	}

	var start uint64 = db_get_block_metadata_by_hash(tip).Height
	var delta uint64 = 0

	var current = COMBInfo.Hash

	for {
		if parent, ok := COMBInfo.Chain[current]; ok {
			delta++
			current = parent
			if current == tip {
				break
			} else {
				continue
			}
		}
		log_panic("push", "trace failed??")
	}
	COMBInfo.Guard.RUnlock()

	log_status("push", "pushing %d blocks...", delta)

	if err = push_blocks(client, start, delta); err != nil {
		log_error("push", "sync failed (%s)", err.Error())
		return
	}

	log_status("push", "%d blocks synced", delta)
}

func push_blocks(client *http.Client, start uint64, delta uint64) (err error) {
	var batch []BlockData
	var block BlockData

	var height = start

	var batch_size = uint64(1000)

	for i := uint64(0); i < delta; i += batch_size {
		batch = make([]BlockData, 0)
		for x := uint64(0); x < batch_size && i+x < delta; x++ {
			if start+i+x != height {
				log_error("push", "missed some blocks")
			}
			height = start + i + x + 1

			block = db_get_block_by_height(height)
			batch = append(batch, block)
		}

		if err = push_blocks_data(client, batch); err != nil {
			return err
		}

		var progress float64 = (float64(i) / float64(delta)) * 100.0
		combcore_set_status(fmt.Sprintf("Pushing (%.2f%%)...", progress))
	}
	return nil
}

func push_encode_block(block BlockData) PushBlockArgs {
	var args PushBlockArgs

	args.Hash = stringify_hex(block.Hash)
	args.Previous = stringify_hex(block.Previous)
	args.Commits = make([]string, 0)

	for _, c := range block.Commits {
		args.Commits = append(args.Commits, stringify_hex(c))
	}

	return args
}

func push_blocks_data(client *http.Client, blocks []BlockData) (err error) {
	var json_blocks []PushBlockArgs = make([]PushBlockArgs, 0)

	for _, b := range blocks {
		json_blocks = append(json_blocks, push_encode_block(b))
	}

	var json_args []byte
	if json_args, err = json.Marshal(json_blocks); err != nil {
		return err
	}

	if _, err = push_rpc(client, "Control.PushBlocks", string(json_args)); err != nil {
		return err
	}

	return nil
}

func push_get_chain_tip(client *http.Client) (hash [32]byte, err error) {
	var response string

	if response, err = push_rpc(client, "Control.GetChainTip", ""); err != nil {
		return [32]byte{}, err
	}

	var hash_string string
	if err = json.Unmarshal([]byte(response), &hash_string); err != nil {
		return [32]byte{}, err
	}

	if hash, err = parse_hex(hash_string); err != nil {
		return [32]byte{}, err
	}

	return hash, nil
}

func push_rpc(client *http.Client, method string, params string) (response string, err error) {
	var req *http.Request
	var resp *http.Response
	var body *strings.Reader = strings.NewReader("{\"jsonrpc\":\"1.0\",\"id\":\"curltext\",\"method\":\"" + method + "\",\"params\":[" + params + "]}")

	req, err = http.NewRequest("POST", fmt.Sprintf("http://%s:%d", PushInfo.IP, PushInfo.Port), body)
	if err != nil {
		log_panic("push", "cannot parse")
	}
	req.Header.Set("Content-Type", "text/plain")

	if resp, err = client.Do(req); err != nil {
		return "", err
	}

	resp_bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return "", err
	}
	resp.Body.Close()

	if len(resp_bytes) == 0 {
		return "", nil
	}

	var result struct {
		Result json.RawMessage `json:"result"`
		Error  string
	}
	if err = json.Unmarshal(resp_bytes, &result); err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf(result.Error)
	}

	return string(result.Result), nil
}
