package client

import (
	"encoding/json"
	"fmt"
)

func PrintJSON(v interface{}) error {
	b, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}
