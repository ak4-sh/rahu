package main

import (
	"fmt"
	"os"

	"rahu/parser"
	"rahu/utils"
)

func main() {
	input := utils.ParseFile("test.py")
	fmt.Printf("Going to parse input string\n%s\n\n\n", input)
	p := parser.New(input) // Pass the same input
	module := p.Parse()
	utils.PrintAST(os.Stdout, module)
}
