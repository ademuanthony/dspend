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

const blockcypherMainnetTransactionApi = "https://api.blockcypher.com/v1/btc/txs/"
const blockcypherTestnetTransactionApi = "https://api.blockcypher.com/v1/btc/test3/txs/"

var blockcypherTransactionApi = blockcypherMainnetTransactionApi

var clientObj client
var rootCmd = &cobra.Command{Use: "dspend"}

const defaultFeeInSatoshi = 400

var createCmd = &cobra.Command{
	Use:   "create-tx",
	Short: "Create a new transaction",
	Run: func(cmd *cobra.Command, args []string) {
		testnet, _ := cmd.Flags().GetBool("testnet")
		if testnet {
			blockcypherTransactionApi = blockcypherTestnetTransactionApi
		}

		debug, _ := cmd.Flags().GetBool("debug")
		clientObj.debugMode = debug

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
		testnet, _ := cmd.Flags().GetBool("testnet")
		if testnet {
			blockcypherTransactionApi = blockcypherTestnetTransactionApi
		}

		debug, _ := cmd.Flags().GetBool("debug")
		clientObj.debugMode = debug

		rawTx, _ := cmd.Flags().GetString("raw-tx")
		if len(rawTx) == 0 {
			fmt.Println("--raw-tx is required")
			os.Exit(1)
		}

		source, _ := cmd.Flags().GetString("source-address")
		if len(rawTx) == 0 {
			fmt.Println("--source-address is required")
			os.Exit(1)
		}

		txDetail, err := clientObj.DecodeRawTransaction(rawTx, source)
		failCleanly(err)
		printJson(txDetail)
	},
}

var modifyCmd = &cobra.Command{
	Use:   "modify-tx",
	Short: "Modify an existing transaction",
	Run: func(cmd *cobra.Command, args []string) {
		rawTx, _ := cmd.Flags().GetString("raw-tx")
		sourceAddress, _ := cmd.Flags().GetString("source-address")

		testnet, _ := cmd.Flags().GetBool("testnet")
		if testnet {
			blockcypherTransactionApi = blockcypherTestnetTransactionApi
		}

		debug, _ := cmd.Flags().GetBool("debug")
		clientObj.debugMode = debug

		modifyTransaction(rawTx, sourceAddress)
	},
}

var sendCmd = &cobra.Command{
	Use:   "send-tx",
	Short: "Send a signed transaction",
	Run: func(cmd *cobra.Command, args []string) {
		testnet, _ := cmd.Flags().GetBool("testnet")
		if testnet {
			blockcypherTransactionApi = blockcypherTestnetTransactionApi
		}

		debug, _ := cmd.Flags().GetBool("debug")
		clientObj.debugMode = debug

		rawTx, _ := cmd.Flags().GetString("raw-tx")
		if len(rawTx) == 0 {
			fmt.Println("--raw-tx is required")
			os.Exit(1)
		}
		sendTransaction(rawTx)
	},
}

func init() {
	err := godotenv.Load()
	failCleanly(err)

	rpcUser = os.Getenv("RPC_USER")
	rpcPass = os.Getenv("RPC_PASS")

	createCmd.Flags().Bool("debug", false, "Run in debug mode")
	createCmd.Flags().Bool("testnet", false, "Run on testnet")
	createCmd.Flags().String("source-address", "", "Source Bitcoin address")
	createCmd.Flags().String("destination-address", "", "Destination Bitcoin address")
	createCmd.Flags().String("fee", "", "Fee in satoshi")

	viewCmd.Flags().Bool("debug", false, "Run in debug mode")
	viewCmd.Flags().Bool("testnet", false, "Run on testnet")
	viewCmd.Flags().StringP("raw-tx", "e", "", "Existing raw transaction in hexadecimal format")
	viewCmd.Flags().StringP("source-address", "a", "", "Source address is required")

	modifyCmd.Flags().Bool("debug", false, "Run in debug mode")
	modifyCmd.Flags().Bool("testnet", false, "Run on testnet")
	modifyCmd.Flags().StringP("raw-tx", "e", "", "Existing raw transaction in hexadecimal format")
	modifyCmd.Flags().String("source-address", "", "Source Bitcoin address")

	sendCmd.Flags().Bool("debug", false, "Run in debug mode")
	sendCmd.Flags().Bool("testnet", false, "Run on testnet")
	sendCmd.Flags().StringP("raw-tx", "s", "", "Raw transaction in hexadecimal format")

	rootCmd.AddCommand(createCmd, viewCmd, modifyCmd, sendCmd)
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

	fmt.Printf("\ntransaction created, here is the raw transaction hex:\n %s\n", txHex)
}

func modifyTransaction(existingRawTxHex, sourceAddress string) {
	if len(sourceAddress) == 0 {
		fmt.Println("--source-address is required")
		os.Exit(1)
	}

	txHex, err := clientObj.ModifyTransaction(existingRawTxHex, sourceAddress)
	failCleanly(err)
	fmt.Println(txHex)
}

func sendTransaction(signedRawTxHex string) {
	fmt.Printf("Sending transaction with raw hex: %s\n", signedRawTxHex)
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
}

type unspentTx struct {
	Address   string  `json:"address"`
	Txid      string  `json:"txid"`
	Vout      int     `json:"vout"`
	Amount    float64 `json:"amount"`
	Spendable bool    `json:"spendable"`
}

type decodedTx struct {
	Txid     string  `json:"txid"`
	Hash     string  `json:"hash"`
	TotalIn  int     `json:"totalid"`
	TotalOut float64 `json:"totalOut"`

	Vin []struct {
		Txid  string  `json:"txid"`
		Vout  int     `json:"vout"`
		Value float64 `json:"value"`
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
	Errors   []struct {
		Txid  string `json:"txid"`
		Error string `json:"error"`
	} `json:"errors"`
}

func (c *client) CreateRawTransaction0(source, destination string, fee int) (unsignedTransactionHash string, err error) {
	if len(source) == 0 {
		return "", fmt.Errorf("source address is required")
	}
	unspentTx, err := c.ListUnspent([]string{source})
	if err != nil {
		return "", err
	}

	if len(unspentTx) != 1 {
		return "", fmt.Errorf("expected 1 unspent tx in %s, got %d", source, len(unspentTx))
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

	if len(destination) == 0 {
		fmt.Println("creating a new output address")
		if c.debugMode {
			fmt.Println("- command: bitcoin-cli getnewaddress")
		}

		cmd := exec.Command(bitcoinCli, "getnewaddress")

		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		destination = strings.Trim(string(output), "\n")
		fmt.Printf("new output address: %s\n", destination)
	}

	fmt.Println("***transaction details***")
	fmt.Printf("destination address: \t %s\n", destination)
	fmt.Printf("source address:\t\t %s\n", source)
	fmt.Printf("input transaction hash:\t %s\n", txid)
	fmt.Printf("input amount:\t\t %.8f\n", amount)
	fmt.Printf("fee:\t\t\t %.8f\n", satoshiToBitcoin(fee))
	fmt.Printf("output amount: \t\t %.8f\n", amountInBTC)

	mapList = append(mapList, map[string]interface{}{
		destination: amountInBTC,
	})
	for _, v := range mapList {
		params = append(params, []interface{}{v})
	}

	if c.debugMode {
		fmt.Printf("- command: bitcoin-cli createrawtransaction \"[{\"txid\":\"%v\",\"vout\":%v}]\" \"[{\"%v\":\"%v\"}]\" \n", txid, vout, destination, amountInBTC)
	}

	res, err := c.Call("createrawtransaction", params, 3)
	failCleanly(err)
	if res.Error != nil {
		err = errors.New(fmt.Sprintf("SIGNRAWTRANSACTIONERROR %v", res.Error))
		return
	}
	if err != nil {
		return "", err
	}

	return res.Result.(string), nil
}

func (c *client) CreateRawTransaction(source string, destination string, fee int) (unsignedTransactionHash string, err error) {
	if len(source) == 0 {
		return "", fmt.Errorf("source address is required")
	}

	unspentTxs, err := c.ListUnspent([]string{source})
	if err != nil {
		return "", err
	}

	if len(unspentTxs) == 0 {
		return "", fmt.Errorf("no unspent transactions for source address %s", source)
	}

	fmt.Printf("using %d unspent tx found in %s\n", len(unspentTxs), source)

	var inputs []map[string]interface{}
	var totalAmount float64

	for _, unspentTx := range unspentTxs {
		txid := unspentTx.Txid
		vout := 1 // You may need to adjust this based on your actual transaction structure

		inputs = append(inputs, map[string]interface{}{
			"txid": txid,
			"vout": vout,
		})

		fmt.Printf("%s:\t%.8f\n", txid, unspentTx.Amount)

		totalAmount += unspentTx.Amount
	}

	if fee == 0 {
		fee = defaultFeeInSatoshi
	}

	// Calculate the total amount less the fee
	amountLessFee := bitcoinToSatoshi(totalAmount) - fee
	if amountLessFee <= 0 {
		return "", fmt.Errorf("invalid amount: %d - %d", bitcoinToSatoshi(totalAmount), amountLessFee)
	}

	amountInBTC := satoshiToBitcoin(amountLessFee)

	if len(destination) == 0 {
		fmt.Println("creating a new output address")
		if c.debugMode {
			fmt.Println("- command: bitcoin-cli getnewaddress")
		}

		cmd := exec.Command(bitcoinCli, "getnewaddress")

		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		destination = strings.Trim(string(output), "\n")
		fmt.Printf("new output address: %s\n", destination)
	}

	fmt.Println("***transaction details***")
	fmt.Printf("destination address: \t %s\n", destination)
	fmt.Printf("source address:\t\t %s\n", source)
	fmt.Printf("total input amount:\t %.8f\n", totalAmount)
	fmt.Printf("fee:\t\t\t %.8f\n", satoshiToBitcoin(fee))
	fmt.Printf("output amount: \t\t %.8f\n", amountInBTC)

	// Pass 'inputs' when calling createrawtransaction
	params := []interface{}{inputs, map[string]interface{}{destination: amountInBTC}}

	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return "", err
	}

	if c.debugMode {
		fmt.Printf("- command: bitcoin-cli createrawtransaction \"%s\" \"[{\"%v\":\"%v\"}]\"\n", inputsJSON, destination, amountInBTC)
	}

	res, err := c.Call("createrawtransaction", params, 3)
	failCleanly(err)
	if res.Error != nil {
		err = errors.New(fmt.Sprintf("CREATERAWTRANSACTIONERROR %v", res.Error))
		return
	}
	if err != nil {
		return "", err
	}

	return res.Result.(string), nil
}

func (c *client) ModifyTransaction(existingRawTxHex, source string) (unsignedTransactionHash string, err error) {
	decodedTx, err := clientObj.DecodeRawTransaction(existingRawTxHex, source)
	if err != nil {
		return "", err
	}

	var inputs []map[string]interface{}

	for _, vin := range decodedTx.Vin {
		inputs = append(inputs, map[string]interface{}{
			"txid": vin.Txid,
			"vout": vin.Vout,
		})
	}

	txData, err := c.GetDataByTransaction(decodedTx.Vin[0].Txid)
	if err != nil {
		return "", err
	}

	var inputAmount, previousOutput float64
	previousOutput = decodedTx.Vout[0].Value

	for _, input := range txData.Outputs {
		if input.Addresses[0] != source {
			continue
		}

		inputAmount = satoshiToBitcoin(input.Value)
	}

	if inputAmount == 0 {
		return "", fmt.Errorf("invalid input or amount")
	}

	destination := decodedTx.Vout[0].ScriptPubKey.Address

	c.displayTransaction(decodedTx, source)

	var modifyFee, modifyDestAddr string

	fmt.Print("\nDo you want to modify the fee? (y/n): ")
	fmt.Scanln(&modifyFee)
	var fee float64
	if modifyFee == "y" {
		var newFee int
		fmt.Print("Enter the new fee (in Satoshis): ")
		fmt.Scanln(&newFee)
		fee = satoshiToBitcoin(newFee)
	} else {
		fee = inputAmount - previousOutput
	}

	fmt.Print("\nDo you want to modify the destination address? (y/n): ")
	fmt.Scanln(&modifyDestAddr)
	if modifyDestAddr == "y" {
		fmt.Println("creating a new output address")
		if c.debugMode {
			fmt.Println("- command: bitcoin-cli getnewaddress")
		}

		cmd := exec.Command(bitcoinCli, "getnewaddress")

		output, err := cmd.Output()
		if err != nil {
			return "", err
		}

		destination = strings.Trim(string(output), "\n")
		fmt.Printf("new output address: %s\n", destination)
	}

	amountInBTC := inputAmount - fee

	if amountInBTC <= 0 {
		return "", fmt.Errorf("invalid amount/fee")
	}

	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return "", err
	}

	params := []interface{}{inputs, map[string]interface{}{destination: amountInBTC}}

	if c.debugMode {
		fmt.Printf("- command: bitcoin-cli createrawtransaction \"%s\" \"[{\"%v\":\"%v\"}]\"\n", inputsJSON, destination, amountInBTC)
	}

	res, err := c.Call("createrawtransaction", params, 3)
	failCleanly(err)
	if res.Error != nil {
		err = errors.New(fmt.Sprintf("SIGNRAWTRANSACTIONERROR %v", res.Error))
		return
	}
	if err != nil {
		return "", err
	}

	return res.Result.(string), nil
}

func (c *client) SignAndSendTx(rawTx string) (string, error) {
	args := []string{"signrawtransactionwithwallet", rawTx}
	fmt.Println("signing transaction...")
	if c.debugMode {
		fmt.Printf("- command: signrawtransactionwithwallet %s \n", rawTx)
	}
	cmd := exec.Command(bitcoinCli, args...)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var signedTx signTransactionResponse
	err = json.Unmarshal(output, &signedTx)
	failCleanly(err)

	if !signedTx.Complete || len(signedTx.Errors) > 0 {
		errMsgg := "error in signing raw transaction\n"
		for _, e := range signedTx.Errors {
			errMsgg += fmt.Sprintf("%s: %s", e.Txid, e.Error)
		}

		return "", fmt.Errorf(errMsgg)
	}

	signedTx.Hex = strings.Trim(signedTx.Hex, "\n")

	fmt.Printf("Signed tx: %s\n", signedTx.Hex)
	// return c.SendRawTransaction(signedTx.Hex)
	return "", nil
}

func (c *client) SendRawTransaction(signedHex string) (transactionID string, err error) {
	var params []interface{}
	params = append(params, signedHex)
	fmt.Println("broadcasting transaction to the network...")
	if c.debugMode {
		fmt.Printf("- command: sendrawtransaction %s \n", signedHex)
	}
	res, err := c.Call("sendrawtransaction", params, 5)
	if err != nil {
		return
	} else if res.Error != nil {
		err = errors.New(fmt.Sprintf("SENDRAWTRANSACTIONERROR %v", res.Error))
		return
	}
	transactionID = res.Result.(string)
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
	requestUrl := blockcypherTransactionApi + txid + "?token=" + os.Getenv("BLOCKCYPHER_API_KEY")
	res, err := http.Get(requestUrl)
	failCleanly(err)

	err = json.NewDecoder(res.Body).Decode(&body)
	if err != nil {
		log.Fatalln("Error =>", err)
	}
	if body.Error != "" {
		return body, fmt.Errorf(body.Error)
	}
	if c.debugMode {
		fmt.Printf("\n GetTransactionByID: %v \n \n", res)
	}
	return
}

func (c *client) DecodeRawTransaction(txHex, source string) (*decodedTx, error) {
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

	for i, vin := range tx.Vin {
		txData, err := c.GetDataByTransaction(vin.Txid)
		if err != nil {
			return nil, err
		}

		for _, input := range txData.Outputs {
			if input.Addresses[0] != source {
				continue
			}

			tx.Vin[i].Value = satoshiToBitcoin(input.Value)
			tx.TotalIn += input.Value
		}
	}

	for _, vout := range tx.Vout {
		tx.TotalOut += (vout.Value)
	}

	return &tx, nil
}

func (c *client) Call(method string, params []interface{}, id uint64) (*clientResponse, error) {
	
	req := &clientRequest{
		Id:     id,
		Method: method,
		Params: params,
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	reqJson := strings.NewReader(string(jsonData))

	res, err := http.Post(c.url, "application/json", reqJson)
	if err != nil {
		return nil, err
	}
	if res.StatusCode > 299 {
		return nil, fmt.Errorf("%v", res.Status)
	}
	
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.Body == nil {
		return nil, errors.New("EMPTY RESPONSE ERROR: there was no response from the server")
	}
	defer res.Body.Close()
	var clientRes clientResponse
	//result := make(map[string]interface{})
	err = json.Unmarshal(body, &clientRes)
	//c.res = result
	if err != nil {
		return nil, err
	}

	return &clientRes, nil
}

func (c *client) displayTransaction(decodedTx *decodedTx, source string) error {
	fee := satoshiToBitcoin(decodedTx.TotalIn) - decodedTx.TotalOut

	fmt.Println("transaction details")
	fmt.Printf("source address: %s\n", source)

	fmt.Println("inputs")
	for _, vin := range decodedTx.Vin {
		fmt.Printf("%s:\t%.8f\n", vin.Txid, vin.Value)
	}
	fmt.Printf("total input:\t\t\t\t\t\t\t\t%.8f\n", satoshiToBitcoin(decodedTx.TotalIn))

	fmt.Println("outputs")
	for _, vout := range decodedTx.Vout {
		fmt.Printf("%s:\t%.8f\n", vout.ScriptPubKey.Address, vout.Value)
	}
	fmt.Printf("fee:\t\t\t\t\t\t%.8f\n", fee)
	fmt.Printf("output amount:\t\t\t\t\t%.8f\n", decodedTx.TotalOut)
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
