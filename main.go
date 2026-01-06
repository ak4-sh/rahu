package main

import (
	"rahu/parser"
	"rahu/utils"
)

func main() {
	input := "result = x + y"
	p := parser.New(input) // Pass the same input

	module := p.Parse()
	utils.PrintAST(module, 0)
}
