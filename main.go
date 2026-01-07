package main

import (
	"rahu/parser"
	"rahu/utils"
	"fmt"
)

func main() {
	input := utils.ParseFile("test.py")
	fmt.Printf("Going to parse input string\n%s\n\n\n", input)
	p := parser.New(input) // Pass the same input
	module := p.Parse()
	utils.PrintAST(module, 0)
}
