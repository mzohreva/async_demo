package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var randomLoadTestCmd = &cobra.Command{
	Use: "random",
	Run: func(cmd *cobra.Command, args []string) {
		randomLoadTestHTTP()
	},
}

func init() {
	loadTestCmd.AddCommand(randomLoadTestCmd)
}

func randomLoadTestHTTP() {
	test := &httpLoadTest{
		setupFunc: func(client *http.Client, url string) error { return nil },
		testFunc: func(client *http.Client, url string, stage Stage) (time.Duration, error) {
			t0 := time.Now()
			err := readWriteHTTP(client, url)
			d := time.Now().Sub(t0)
			return d, err
		},
		cleanupFunc: func(client *http.Client, url string) {},
	}
	loadTest("random - http", test)
}

type Input struct {
	Message string `json:"message"`
}

type Output struct {
	Message string `json:"message"`
	Hash    string `json:"hash"`
}

func readWriteHTTP(client *http.Client, url string) error {
	input := Input{Message: string(makeRandomBytes())}
	buf, err := json.Marshal(input)
	if err != nil {
		return err
	}
	resp, err := client.Post(url, "binary/garbage", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var output Output
	json.Unmarshal(body, &output)
	if err != nil {
		return err
	}
	if input.Message != output.Message {
		return fmt.Errorf("output message does not match!")
	}
	return nil
}

func makeRandomBytes() []byte {
	size := 16 + rand.Intn(1008)
	val := byte(32 + rand.Intn(127-32))
	return bytes.Repeat([]byte{val}, size)
}
