package main

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"syscall/js"

	"fmt"

	"encoding/xml"

	"github.com/nicholasblaskey/svg-rasterizer/board"
)

type rasterizer struct {
	board *board.Board
}

type Node struct {
	XMLName xml.Name
	Content []byte `xml:",innerxml"`
	Nodes   []Node `xml:",any"`
}

func New(canvas js.Value, filePath string) (*rasterizer, error) {
	fileString := getFile(filePath)

	buf := bytes.NewBuffer([]byte(fileString))
	dec := xml.NewDecoder(buf)

	var n Node
	err := dec.Decode(&n)
	if err != nil {
		panic(err)
	}

	walk([]Node{n}, func(n Node) bool {
		fmt.Println(n.XMLName.Local)
		/*
			if n.XMLName.Local == "p" {
				fmt.Println(string(n.Content))
			}
		*/
		return true
	})

	fmt.Println(filePath, fileString)

	/*
		var xmlFile map[string]interface{}
		err := xml.Unmarshal([]byte(fileString), &xmlFile)
		if err != nil {
			panic(err)
		}

		fmt.Println(xmlFile)
	*/

	return &rasterizer{}, nil
}

func walk(nodes []Node, f func(Node) bool) {
	for _, n := range nodes {
		if f(n) {
			walk(n.Nodes, f)
		}
	}
}

func getFile(filePath string) string {
	loc := js.Global().Get("location")
	url := loc.Get("protocol").String() + "//" +
		loc.Get("hostname").String() + ":" +
		loc.Get("port").String()

	resp, err := http.Get(url + filePath)
	if err != nil {
		panic(err)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	s := string(b)
	return strings.ReplaceAll(s, "\r", "")
}

func main() {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", "webgl")

	New(canvas, "/svg/test1.svg")

	canvas.Set("height", 800)
	canvas.Set("width", 800)
	b, err := board.New(canvas)
	if err != nil {
		panic(err)
	}
	b.EnablePixelInspector(true)

	fmt.Println("starting", rand.Int31n(256))

	/*
		data := []byte{}
		white := false
		for i := 0; i < b.Width; i++ {
			white = i%2 == 0
			for j := 0; j < b.Height; j++ {
				if white {
					data = append(data, 255)
				} else {
					data = append(data, 0)
				}
				white = !white
			}
		}
		b.SetPixels(data)
	*/

	data := []byte{}
	for i := 0; i < b.Width/10; i++ {
		col1, col2 := byte(rand.Int31n(256)), byte(rand.Int31n(256))

		for k := 0; k < 10; k++ {

			for j := 0; j < b.Height/2; j++ {
				data = append(data, col1)
			}
			for j := b.Height / 2; j < b.Height; j++ {
				data = append(data, col2)
			}
		}
	}
	b.SetPixels(data)

	<-make(chan bool) // Prevent program from exiting
}
