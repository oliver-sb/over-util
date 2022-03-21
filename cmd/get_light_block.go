package cmd

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"
	"github.com/valyala/fasthttp"

	"github.com/cosmos/btcutil/bech32"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/gogo/protobuf/proto"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var getLightBlockCmd = &cobra.Command{
	Use:   "get-light-block",
	Short: "Get Light Block Data (base64 encoding to protobuf binary",
	Run: func(cmd *cobra.Command, args []string) {
		runGetLightBlock()
	},
}

func getSignedHeader(data []byte) *types.SignedHeader {
	var resultBlock struct {
		BlockID types.BlockID `json:"block_id"`
		Block   struct {
			types.Header `json:"header"`
			LastCommit   *types.Commit `json:"last_commit"`
		} `json:"block"`
	}
	err := legacy.Cdc.UnmarshalJSON(data, &resultBlock)
	if err != nil {
		panic(err)
	}

	block := resultBlock.Block

	return &types.SignedHeader{
		Header: &block.Header,
		Commit: block.LastCommit,
	}
}

func getValidatorSet(data []byte) *types.ValidatorSet {
	var resultValset struct {
		Result struct {
			Validators []json.RawMessage `json:"validators"`
		} `json:"result"`
	}
	err := legacy.Cdc.UnmarshalJSON(data, &resultValset)
	if err != nil {
		log.Println("Unmarshal 1", "error", err, "data", resultValset)
		return nil
	}

	valsets := []*types.Validator{}
	for _, d := range resultValset.Result.Validators {
		var v types.Validator = types.Validator{}

		var val struct {
			Address          string             `json:"address"`
			PubKey           cryptotypes.PubKey `json:"pub_key,omitempty"`
			VotingPower      int64              `json:"voting_power,string"`
			ProposerPriority int64              `json:"proposer_priority,string"`
		}
		err = legacy.Cdc.UnmarshalJSON(d, &val)
		if err != nil {
			log.Println("Unmarshal 2", "error", err)
			return nil
		}
		v.PubKey, err = cryptocodec.ToTmPubKeyInterface(val.PubKey)
		if err != nil {
			log.Println("Unmarshal 3", "error", err)
			return nil
		}
		_, v.Address, _ = DecodeAndConvert(string(val.Address))
		v.VotingPower = val.VotingPower
		v.ProposerPriority = val.ProposerPriority

		//jsonStr, _ := json.Marshal(v)
		//logger.Info("%s - %+v\n", string(v.Address), jsonStr)

		valsets = append(valsets, &v)
	}

	return &types.ValidatorSet{
		Validators: valsets,
		Proposer:   valsets[0],
	}
}

func getData(url string) []byte {
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	client := &fasthttp.Client{}
	defer client.CloseIdleConnections()
	defer resp.ConnectionClose()

	err := client.DoTimeout(req, resp, 10*time.Second)
	if err != nil {
		log.Printf("Error on RequestRPC : %+v\n", err)
		return nil
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		log.Printf("StatusCode not OK : %s\n", resp.String())
		return nil
	}

	return resp.Body()
}

func getValidators(url string) []*types.Validator {
	valsetData := getData(url)
	return getValidatorSet(valsetData).Validators
}

func DecodeAndConvert(bech string) (string, []byte, error) {
	hrp, data, err := bech32.Decode(bech, 1023)
	if err != nil {
		return "", nil, fmt.Errorf("decoding bech32 failed: %w", err)
	}

	converted, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", nil, fmt.Errorf("decoding bech32 failed: %w", err)
	}

	return hrp, converted, nil
}

func runGetLightBlock() {
	log.Println("Run Over")

	host := "https://bombay.stakesystems.io"
	//host := "http://localhost:1317"

	blockData := getData(host + "/blocks/latest")
	signedHeader := getSignedHeader(blockData)
	url0 := fmt.Sprintf("%s/blocks/%d", host, signedHeader.Commit.Height)
	blockData = getData(url0)
	signedHeader2 := getSignedHeader(blockData)
	signedHeader.Header = signedHeader2.Header

	protoHeader := signedHeader.ToProto()

	byteData, err := proto.Marshal(protoHeader)
	if err != nil {
		log.Println("", "error", err)
		return
	}

	header_proto := base64.RawStdEncoding.EncodeToString(byteData)
	log.Println("", "encoded", len(header_proto), "byte data", len(byteData))

	url1 := fmt.Sprintf("%s/validatorsets/%d?page=1", host, signedHeader.Header.Height)
	valset1 := getValidators(url1)

	//url2 := fmt.Sprintf("%s/validatorsets/%d?page=2", host, signedHeader.Header.Height)
	//valset2 := getValidators(url2, logger)
	valset2 := []*types.Validator{} // Test Net 은 Validator 가 많지 않음

	valset := types.ValidatorSet{
		Validators: append(valset1, valset2...),
		Proposer:   valset1[0],
	}

	valsetProto, err := valset.ToProto()
	if err != nil {
		log.Println("ToProto", "error", err)
		return
	}
	byteData, err = proto.Marshal(valsetProto)
	if err != nil {
		log.Println("Marshal", "error", err)
		return
	}

	valset_proto := base64.RawStdEncoding.EncodeToString(byteData)
	log.Println("", "encoded", len(valset_proto), "byte data", len(byteData), "url", url1)

	log.Println("Validation",
		"ValidatorHash", hex.EncodeToString(signedHeader.Header.ValidatorsHash),
		"ValSetHash", hex.EncodeToString(valset.Hash()),
		"Equality", hex.EncodeToString(signedHeader.Header.ValidatorsHash) == hex.EncodeToString(valset.Hash()),
		"Length", len(valset.Validators),
	)

	//fmt.Println(header_proto)
	//fmt.Println(valset_proto)

	lightBlock := types.LightBlock{
		SignedHeader: signedHeader,
		ValidatorSet: &valset,
	}

	lightBlockProto, err := lightBlock.ToProto()
	if err != nil {
		log.Println("LightToProto", "error", err)
		return
	}
	lightBlockData, err := proto.Marshal(lightBlockProto)
	if err != nil {
		log.Println("Marshal", "error", err)
		return
	}

	lightBlockEncoded := base64.RawStdEncoding.EncodeToString(lightBlockData)
	//sample := valset.Validators[0].Address
	//fmt.Println("[Oliver]", sample, len(sample), string(sample), string(valset.Validators[1].Address))
	//fmt.Println("[Oliver]", string(sample[31:]), len(sample[31:]))
	//prefix, address, err := DecodeAndConvert(string(sample))
	//fmt.Println("[Oliver]", prefix, address, string(address), len(address))
	//fmt.Println("[Oliver]", hex.DecodeString(valset.Validators[0].Address))
	log.Println("", "encoded", len(lightBlockEncoded), "byte data", len(lightBlockData), "url", url1, "prefix", hex.EncodeToString(lightBlockData[:10]))
	log.Println("Info", "Commit Height", signedHeader.Commit.Height, "Height", signedHeader.Height)

	pb := new(tmproto.LightBlock)

	err = proto.Unmarshal(lightBlockData, pb)
	if err != nil {
		log.Println("Fail To Unmarshal", "Error", err)
		return
	}

	lb, err := types.LightBlockFromProto(pb)
	if err != nil {
		log.Println("Fail To LightBlockFromProto", "Error", err)
		return
	}

	err = lb.SignedHeader.ValidateBasic(lb.SignedHeader.ChainID)
	if err != nil {
		log.Println("Fail To ValidateBasic", "Error", err)
		return
	}

	err = lb.ValidatorSet.VerifyCommitLight(lb.SignedHeader.ChainID, lb.SignedHeader.Commit.BlockID, lb.SignedHeader.Height, lb.SignedHeader.Commit)
	if err != nil {
		log.Println("Fail To Verify Commit Light", "Error", err)
		return
	}

	fmt.Println(lightBlockEncoded)
}
