package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const port int = 18332 // 8334
const ipAddress string = "127.0.0.1"
var rpcUser string = "user"
var rpcPass string = "123321"

const bitcoinCli = "bitcoin-cli"

// const blockcypherTransactionApi = "https://api.blockcypher.com/v1/btc/txs/"
const blockcypherTransactionApi = "https://api.blockcypher.com/v1/btc/test3/txs/"

var clientObj client
var rootCmd = &cobra.Command{Use: "dspend"}

const defaultFeeInSatoshi = 219

var createCmd = &cobra.Command{
	Use:   "create-tx",
	Short: "Create a new transaction",
	Run: func(cmd *cobra.Command, args []string) {
		sourceAddress, _ := cmd.Flags().GetString("source-address")
		destinationAddress, _ := cmd.Flags().GetString("destination-address")
		fee, _ := cmd.Flags().GetInt("fee")

		createTransaction(sourceAddress, destinationAddress, fee)
	},
}

var viewCmd = &cobra.Command{
	Use:   "view-tx",
	Short: "View transaction",
	Run: func(cmd *cobra.Command, args []string) {
		rawTx, _ := cmd.Flags().GetString("raw-tx")
		if len(rawTx) == 0 {
			fmt.Println("--raw-tx is required")
			os.Exit(1)
		}
		txDetail, err := clientObj.DecodeRawTransaction(rawTx)
		failCleanly(err)
		printJson(txDetail)
	},
}

var modifyCmd = &cobra.Command{
	Use:   "modify-tx",
	Short: "Modify an existing transaction",
	Run: func(cmd *cobra.Command, args []string) {
		rawTx, _ := cmd.Flags().GetString("raw-tx")
		destinationAddress, _ := cmd.Flags().GetString("new-destination")
		sourceAddress, _ := cmd.Flags().GetString("source-address")
		fee, _ := cmd.Flags().GetInt("new-fee")

		modifyTransaction(rawTx, sourceAddress, destinationAddress, fee)
	},
}

var sendCmd = &cobra.Command{
	Use:   "send-tx",
	Short: "Send a signed transaction",
	Run: func(cmd *cobra.Command, args []string) {
		rawTx, _ := cmd.Flags().GetString("signed-raw-tx")
		if len(rawTx) == 0 {
			fmt.Println("--raw-tx is required")
			os.Exit(1)
		}
		sendTransaction(rawTx)
	},
}

var webCmd = &cobra.Command{
	Use:   "run-web",
	Short: "Web mode",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Web mode")
	},
}

func init() {
	err := godotenv.Load()
	failCleanly(err)

	rpcUser = os.Getenv("RPC_USER")
	rpcPass = os.Getenv("RPC_PASS")

	createCmd.Flags().String("source-address", "", "Source Bitcoin address")
	createCmd.Flags().String("destination-address", "", "Destination Bitcoin address")
	createCmd.Flags().String("fee", "", "Fee in satoshi")

	viewCmd.Flags().StringP("raw-tx", "e", "", "Existing raw transaction in hexadecimal format")

	modifyCmd.Flags().StringP("raw-tx", "e", "", "Existing raw transaction in hexadecimal format")
	modifyCmd.Flags().String("source-address", "", "Source Bitcoin address")
	modifyCmd.Flags().String("new-destination", "", "Destination Bitcoin address")
	modifyCmd.Flags().Int("new-fee", 0, "Fee in satoshi")

	sendCmd.Flags().StringP("signed-raw-tx", "s", "", "Signed raw transaction in hexadecimal format")

	rootCmd.AddCommand(createCmd, viewCmd, modifyCmd, sendCmd, webCmd)
}

func main() {
	clientObj = client{
		debugMode: true,
		url:       "http://" + rpcUser + ":" + rpcPass + "@" + ipAddress + ":" + strconv.Itoa(port),
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func createTransaction(sourceAddress, destinationAddress string, fee int) {
	txHex, err := clientObj.CreateRawTransaction(sourceAddress, destinationAddress, fee)
	failCleanly(err)
	fmt.Println(txHex)
}

func modifyTransaction(existingRawTxHex, sourceAddress, destination string, fee int) {
	if len(sourceAddress) == 0 {
		fmt.Println("--source-address is required")
		os.Exit(1)
	}

	if len(destination) == 0 {
		fmt.Println("--destination-address is required")
		os.Exit(1)
	}

	txHex, err := clientObj.ModifyTransaction(existingRawTxHex, sourceAddress, destination, fee)
	failCleanly(err)
	fmt.Println(txHex)
}

func sendTransaction(signedRawTxHex string) {
	fmt.Printf("Sending signed transaction with raw hex: %s\n", signedRawTxHex)
	hash, err := clientObj.SignAndSendTx(signedRawTxHex)
	failCleanly(err)
	fmt.Println("Transaction sent. Hash:")
	fmt.Println(hash)
}

type clientResponse struct {
	Id     uint64      `json:"id"`
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

type clientRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
	Id     uint64      `json:"id"`
}

type client struct {
	debugMode bool

	url     string
	req     *clientRequest
	res     *clientResponse
	reqJson *strings.Reader
}

type unspentTx struct {
	Address   string  `json:"address"`
	Txid      string  `json:"txid"`
	Vout      int     `json:"vout"`
	Amount    float64 `json:"amount"`
	Spendable bool    `json:"spendable"`
}

type decodedTx struct {
	Txid string `json:"txid"`
	Hash string `json:"hash"`
	Vin  []struct {
		Txid string `json:"txid"`
		Vout int    `json:"vout"`
	} `json:"vin"`
	Vout []struct {
		Value        float64 `json:"value"`
		ScriptPubKey struct {
			Hex     string `json:"hex"`
			Address string `json:"address"`
		} `json:"scriptPubKey"`
	}
}

type blockcypherTx struct {
	Error     string `json:"error"`
	BlockHash string `json:"block_hash"`
	Inputs    []struct {
		Addresses []string `json:"addresses"`
		PrevHash  string   `json:"prev_hash"`
	} `json:"inputs"`

	Outputs []struct {
		Addresses []string `json:"addresses"`
		Value     int      `json:"value"`
	} `json:"outputs"`
}

type signTransactionResponse struct {
	Hex      string `json:"hex"`
	Complete bool   `json:"complete"`
}

func (c *client) CreateRawTransaction(source, destination string, fee int) (unsignedTransactionHash string, err error) {
	unspentTx, err := c.ListUnspent([]string{source})
	if err != nil {
		return "", err
	}

	var txid string = unspentTx[0].Txid
	var amount float64 = unspentTx[0].Amount

	var params []interface{}
	var mapList []map[string]interface{}

	if fee == 0 {
		fee = defaultFeeInSatoshi
	}
	amountLessFee := bitcoinToSatoshi(amount) - fee
	if amountLessFee <= 0 {
		return "", fmt.Errorf("invalid amount: %d - %d", bitcoinToSatoshi(amount), amountLessFee)
	}
	amountInBTC := satoshiToBitcoin(amountLessFee)

	vout := 1

	mapList = append(mapList, map[string]interface{}{
		"txid": txid,
		"vout": vout,
	})
	mapList = append(mapList, map[string]interface{}{
		destination: amountInBTC,
	})
	for _, v := range mapList {
		params = append(params, []interface{}{v})
	}

	if c.debugMode {
		fmt.Printf("command: bitcoin-cli createrawtransaction \"[{\"txid\":\"%v\",\"vout\":%v}]\" \"[{\"%v\":\"%v\"}]\" \n", txid, vout, destination, amountInBTC)
	}

	err = c.Call("createrawtransaction", params, 3)
	failCleanly(err)
	if c.res.Error != nil {
		err = errors.New(fmt.Sprintf("SIGNRAWTRANSACTIONERROR %v", c.res.Error))
		return
	}
	if err != nil {
		return "", err
	}

	return c.res.Result.(string), nil
}

func (c *client) ModifyTransaction(existingRawTxHex, source, destination string, fee int) (unsignedTransactionHash string, err error) {
	decodedTx, err := clientObj.DecodeRawTransaction(existingRawTxHex)
	if err != nil {
		return "", err
	}

	var params []interface{}
	var mapList []map[string]interface{}

	txid := decodedTx.Vin[0].Txid
	vout := decodedTx.Vin[0].Vout

	mapList = append(mapList, map[string]interface{}{
		"txid": txid,
		"vout": vout,
	})
	amountInBTC := decodedTx.Vout[0].Value
	if fee > 0 {
		txData, err := c.GetDataByTransaction(decodedTx.Vin[0].Txid)
		if err != nil {
			return "", err
		}

		var inputAmount float64
		for _, input := range txData.Outputs {
			if input.Addresses[0] != source {
				continue
			}

			inputAmount = satoshiToBitcoin(input.Value)
		}

		amountInBTC = inputAmount - satoshiToBitcoin(fee)

		if amountInBTC <= 0 {
			return "", fmt.Errorf("invalid amount")
		}

	}

	mapList = append(mapList, map[string]interface{}{
		destination: amountInBTC,
	})
	for _, v := range mapList {
		params = append(params, []interface{}{v})
	}

	if c.debugMode {
		fmt.Printf("command: bitcoin-cli createrawtransaction \"[{\"txid\":\"%v\",\"vout\":%v}]\" \"[{\"%v\":\"%v\"}]\" \n", txid, vout, destination, amountInBTC)
	}

	// bitcoin-cli createrawtransaction "[{\"txid\":\"fb789e9f2f41091ab73448b6d678f46c7336909c8ce388a197b515ffb5d29368\",\"vout\":1}]" "[{\"tb1qmj6a2r4c78mzzn9kah83s4hlsyzfm2arsv8ft5\":\"0.0151094\"}]"
	err = c.Call("createrawtransaction", params, 3)
	failCleanly(err)
	if c.res.Error != nil {
		err = errors.New(fmt.Sprintf("SIGNRAWTRANSACTIONERROR %v", c.res.Error))
		return
	}
	if err != nil {
		return "", err
	}

	return c.res.Result.(string), nil
}

func (c *client) SignAndSendTx(rawTx string) (string, error) {
	args := []string{"signrawtransactionwithwallet", rawTx}
	if c.debugMode {
		fmt.Printf("command: signrawtransactionwithwallet %s \n", rawTx)
	}
	cmd := exec.Command(bitcoinCli, args...)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var signedTx signTransactionResponse
	err = json.Unmarshal(output, &signedTx)
	failCleanly(err)

	fmt.Printf("Signed tx: %s\n", signedTx.Hex)
	return c.SendRawTransaction(signedTx.Hex)
}

func (c *client) SendRawTransaction(signedHex string) (transactionID string, err error) {
	var params []interface{}
	params = append(params, signedHex)
	if c.debugMode {
		fmt.Printf("command: sendrawtransaction %s \n", signedHex)
	}
	err = c.Call("sendrawtransaction", params, 5)
	if err != nil {
		return
	} else if c.res.Error != nil {
		err = errors.New(fmt.Sprintf("SENDRAWTRANSACTIONERROR %v", c.res.Error))
		return
	}
	transactionID = c.res.Result.(string)
	return
}

func (c client) ListUnspent(address []string) ([]unspentTx, error) {
	addressJson, err := json.Marshal(address)
	if err != nil {
		return nil, err
	}
	args := []string{"listunspent", "0", "9999999", string(addressJson)}
	cmd := exec.Command(bitcoinCli, args...)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var txs []unspentTx
	err = json.Unmarshal(output, &txs)
	if err != nil {
		return nil, err
	}

	return txs, nil
}

func (c *client) GetDataByTransaction(txid string) (body blockcypherTx, err error) {
	requestUrl := blockcypherTransactionApi + txid
	res, err := http.Get(requestUrl)
	failCleanly(err)

	err = json.NewDecoder(res.Body).Decode(&body)
	if err != nil {
		log.Fatalln("Error =>", err)
	}
	if body.Error != "" {
		return body, fmt.Errorf(body.Error)
	}
	fmt.Printf(" success! \n")
	if c.debugMode {
		fmt.Printf("\n GetTransactionByID: %v \n \n", res)
	}
	return
}

func (c *client) DecodeRawTransaction(txHex string) (*decodedTx, error) {
	args := []string{"decoderawtransaction", txHex}
	cmd := exec.Command(bitcoinCli, args...)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var tx decodedTx
	err = json.Unmarshal(output, &tx)
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func (c *client) WriteRequest(method string, params interface{}, id uint64) error {
	c.req = &clientRequest{
		Id:     id,
		Method: method,
		Params: params,
	}
	jsonData, err := json.Marshal(c.req)
	if err != nil {
		return err
	}
	c.reqJson = strings.NewReader(string(jsonData))
	return nil
}

func (c *client) ReadResponseBody(res *http.Response) error {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.Body == nil {
		return errors.New("EMPTY RESPONSE ERROR: there was no response from the server")
	}
	defer res.Body.Close()
	//result := make(map[string]interface{})
	err = json.Unmarshal(body, &c.res)
	//c.res = result
	if err != nil {
		return err
	}
	return nil
}

func (c *client) Call(method string, params []interface{}, id uint64) error {
	err := c.WriteRequest(method, params, id)
	if err != nil {
		return err
	}
	res, err := http.Post(c.url, "application/json", c.reqJson)
	if err != nil {
		return err
	}
	if res.StatusCode > 299 {
		return fmt.Errorf("%v", res.Status)
	}
	err = c.ReadResponseBody(res)
	if err != nil {
		return err
	}
	return nil
}

func failCleanly(err error) {
	if err != nil {
		fmt.Println("")
		log.Fatalln("Error!!:", err)
	}
}

func bitcoinToSatoshi(btc float64) int {
	return int(btc * float64(100000000))
}

func satoshiToBitcoin(satoshi int) float64 {
	return float64(satoshi) / 100000000

}

func printJson(data interface{}) {
	output, err := json.MarshalIndent(data, "", "  ")
	failCleanly(err)
	fmt.Println(string(output))
}
