package cmd

import (
	"context"
	"encoding/hex"
	"fmt"

	storetype "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/crypto/merkle"
	lightClient "github.com/tendermint/tendermint/light/rpc"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
	"github.com/spf13/cobra"
)

var getTerraProofCmd = &cobra.Command{
	Use:   "get-terra-proof",
	Short: "Get Terra Proof (hex encoding)",
	Run: func(cmd *cobra.Command, args []string) {
		runGetTerraProof()
	},
}

var height *int64

func init() {
	height = getTerraProofCmd.Flags().Int64("height", 0, "Block Height (Block Number)")
	getTerraProofCmd.MarkFlagRequired("height")
}

func CosmWasmLengthPrefix(bz []byte) ([]byte, error) {
	bzLen := len(bz)
	f := byte(0)
	if bzLen > 255 {
		f = uint8(bzLen / 255)
		bzLen = bzLen - int(f)*255
	}
	return append([]byte{f, byte(bzLen)}, bz...), nil
}

func runGetTerraProof() {
	//client, err := http.New("https://terra-rpc.easy2stake.com:443", "/websocket")
	client, err := http.New("https://bombay.stakesystems.io:2053", "/websocket")
	if err != nil {
		panic(err)
	}

	//var height int64 = 6866191
	//result, err := client.BlockResults(context.Background(), &height)
	//if err != nil {
	//	panic(err)
	//}

	//_ = result
	//fmt.Println(result)

	sdkConfig := sdk.GetConfig()
	sdkConfig.SetBech32PrefixForAccount("terra", "terrapub")
	data, err := sdk.AccAddressFromBech32("terra1mjuymqcu3gmyc8uj7njmpdqrayvv7vvuqul65f")
	// hex.DecodeString("052c7465727261316d6a75796d71637533676d796338756a376e6a6d70647172617976763776767571756c363566746f6b656e5f696e666f")
	if err != nil {
		panic(err)
	}
	queryData1 := append([]byte{0x05}, address.MustLengthPrefix(data.Bytes())...)
	//data, err := hex.DecodeString("0514e520243e27e0cb61739fe8e38ee4f7068141da9c746f6b656e5f696e666f")
	//data, err := hex.DecodeString("0514f42dd62164b962bfb5f7a5eb96249d933647db3e7374617465")
	if err != nil {
		panic(err)
	}

	walletAddr, _ := sdk.AccAddressFromBech32("terra1wgz957ynt43dxc4n45pem2gvaeg8xhxrrh69h2")
	prefix, _ := CosmWasmLengthPrefix([]byte("balance"))
	fmt.Println(prefix)
	// addrWithLen, _ := CosmWasmLengthPrefix([]byte("terra1wgz957ynt43dxc4n45pem2gvaeg8xhxrrh69h2"))
	// fmt.Println(addrWithLen)

	k := append(prefix, walletAddr...)
	queryData := append(queryData1, k...)

	path := "/store/wasm/key"
	result2, err := client.ABCIQueryWithOptions(context.Background(), path, queryData, rpcclient.ABCIQueryOptions{Height: *height - 1, Prove: true})
	if err != nil {
		panic(err)
	}

	resp := result2.Response

	//fmt.Printf("%+v\n", result2)

	new_height := resp.Height + 1
	block, err := client.Block(context.Background(), &new_height)
	appHash := block.Block.AppHash
	prt := merkle.DefaultProofRuntime()
	keyPathFunc := lightClient.DefaultMerkleKeyPathFn()
	kp, _ := keyPathFunc(path, resp.Key)
	prt.RegisterOpDecoder("ics23:iavl", storetype.CommitmentOpDecoder)
	prt.RegisterOpDecoder("ics23:simple", storetype.CommitmentOpDecoder)
	err = prt.VerifyValue(result2.Response.ProofOps, appHash, kp.String(), result2.Response.Value)

	if err != nil {
		panic(err)
	}

	simpleProof, err := proto.Marshal(&resp.ProofOps.Ops[1])
	if err != nil {
		panic(err)
	}
	wasmProof, err := proto.Marshal(&resp.ProofOps.Ops[0])
	if err != nil {
		panic(err)
	}

	fmt.Printf("[simple proof] %s\n", hex.EncodeToString(simpleProof))
	fmt.Printf("[wasm proof] %s\n", hex.EncodeToString(wasmProof))
	fmt.Printf("[path] %s\n", path)
	fmt.Printf("[key] %s\n", hex.EncodeToString(resp.Key))
	fmt.Printf("[value] %s\n", hex.EncodeToString(resp.Value))
}
