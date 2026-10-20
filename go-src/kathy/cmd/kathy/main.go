// Command kathy converts a `.chw.tex` page into its Kathy document on stdout.
//
// TODO: the conversion is not written yet; the command reads its argument and
// stops.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: kathy <page.chw.tex>")
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "kathy: conversion of %s is not implemented yet\n", os.Args[1])
	os.Exit(1)
}
