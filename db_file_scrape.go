package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
)

const graphPath = "/Users/jacobmikesell/.logseq/graphs/logseq_local_++Users++jacobmikesell++Documents++zet.transit"

type atom []any
type block struct {
	ID      int
	Ident   string
	Content string
	LongID  int
}

func main() {
	fmt.Println("hello world")
	bs, err := ioutil.ReadFile(graphPath)
	if err != nil {
		panic(err)
	}
	as := atom{}
	err = json.Unmarshal(bs, &as)
	if err != nil {
		panic(err)
	}
	blocks := as[1].([]any)[4].([]any)[1].([]any)
	for _, b := range blocks {
		data := b.([]any)[1].([]any)
		if _, ok := data[2].(string); !ok {
			// fmt.Println(data)
			continue
		}
		if strings.Contains(data[2].(string), "639bf695-a412-43e4-ac38-54741dd68169") {
			fmt.Println(b)
		}
	}
}