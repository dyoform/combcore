package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func rest_trace_chain(client *http.Client, url string, target [32]byte, length uint64) (chain [][32]byte, err error) {
	var raw_json json.RawMessage
	var headers []struct {
		Height            uint64
		Previousblockhash string
	}

	//keep tracing the chain back from the tip until we find a block thats known (in history)
	var hash [32]byte = target

	for {
		COMBInfo.Guard.RLock()
		_, ok := COMBInfo.Chain[hash]
		COMBInfo.Guard.RUnlock()
		if ok {
			break
		}

		chain = append(chain, hash)
		if raw_json, err = btc_rest_call(client, fmt.Sprintf("%s/headers/1/%x.json", url, hash)); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw_json, &headers); err != nil {
			return nil, fmt.Errorf("header is gibberish (%s) got (%s)", err.Error(), string(raw_json))
		}
		if len(headers) == 0 {
			return nil, fmt.Errorf("cannot find header for %X", hash)
		}
		if hash, err = parse_hex(headers[0].Previousblockhash); err != nil {
			return nil, err
		}

		LogInfo("rest", "tracing %X", hash)

		//just for the end user, this wont factor in any reorgs
		var progress float64 = (float64(len(chain)) / float64(length)) * 100.0
		combcore_set_status(fmt.Sprintf("Tracing (%.2f%%)...", progress))
	}

	//reverse the chain so older blocks are mined first
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain, nil
}

func rest_get_block_range(client *http.Client, url string, target [32]byte, length uint64, out chan<- BlockData) (err error) {
	defer close(out)
	var chain [][32]byte
	var block BlockData

	//gets a list of blocks that connect the target to a known block (does not have to be the current chain tip)
	//every block in this list is unknown to combcore
	if chain, err = rest_trace_chain(client, url, target, length); err != nil {
		return err
	}

	for i, h := range chain {
		if block, err = rest_get_block(client, url, h); err != nil {
			return err
		}
		var progress float64 = (float64(i) / float64(length)) * 100.0
		combcore_set_status(fmt.Sprintf("Mining (%.2f%%)...", progress))
		out <- block
	}

	return nil
}

func rest_get_block(client *http.Client, url string, hash [32]byte) (block BlockData, err error) {
	var raw_data []byte
	var raw_block *BlockData = new(BlockData)

	if raw_data, err = btc_rest_call(client, fmt.Sprintf("%s/block/%x.bin", url, hash)); err != nil {
		return block, err
	}

	btc_parse_block(raw_data, raw_block)

	if raw_block.Hash != hash {
		LogPanic("rest", "recieved wrong block %X != %X", raw_block.Hash, hash)
	}

	block.Hash = raw_block.Hash
	block.Previous = raw_block.Previous
	block.Commits = raw_block.Commits

	return block, nil
}

func rest_get_chains(client *http.Client, url string) (chain ChainInfo, err error) {
	var raw_json json.RawMessage
	var raw_chain struct {
		Blocks        uint64
		Headers       uint64
		BestBlockHash string
	}

	if raw_json, err = btc_rest_call(client, fmt.Sprintf("%s/chaininfo.json", url)); err != nil {
		return chain, err
	}

	if err = json.Unmarshal(raw_json, &raw_chain); err != nil {
		return chain, fmt.Errorf("chain data is gibberish (%s) got (%s)", err.Error(), string(raw_json))
	}

	if chain.TopHash, err = parse_hex(raw_chain.BestBlockHash); err != nil {
		return chain, err
	}

	chain.Height = raw_chain.Blocks
	chain.KnownHeight = raw_chain.Headers

	return chain, nil
}

func btc_rest_call(client *http.Client, url string) (json.RawMessage, error) {
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Content-Type", "text/plain")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("response not OK")
	}

	response_data, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return nil, err
	}

	return response_data, nil
}
