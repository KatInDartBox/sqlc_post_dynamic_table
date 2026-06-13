package gen

import (
	"encoding/json"
	"fmt"
)

func Log(name string, data any) {
	p, _ := json.MarshalIndent(data, "", " ")
	s := "==========="
	fmt.Println(s + name + s)
	fmt.Println(string(p))
	fmt.Println(s + name + s)
}
