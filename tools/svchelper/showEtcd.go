package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func showEtcd() {
	resp, err := http.Post("http://192.168.1.110:2379/v3/kv/range", "application/json",
		bytes.NewReader([]byte(`{"key":"L3NlcnZpY2VzLw==","range_end":"L3NlcnZpY2VzMA=="}`)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "etcd query failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Kvs []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"kvs"`
	}
	json.Unmarshal(body, &result)

	if len(result.Kvs) == 0 {
		fmt.Println("No services registered in etcd")
		return
	}
	fmt.Println("Registered services:")
	for _, kv := range result.Kvs {
		k, _ := base64.StdEncoding.DecodeString(kv.Key)
		v, _ := base64.StdEncoding.DecodeString(kv.Value)
		fmt.Printf("  %s -> %s\n", string(k), string(v))
	}
}
