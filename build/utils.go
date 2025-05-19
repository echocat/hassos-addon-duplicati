package main

import (
	"strconv"
)

func recordmln(k, v string) string {
	return k + "<<EOF\n" + v + "EOF\n"
}

func recordln(k, v string) string {
	return k + "=" + strconv.Quote(v) + "\n"
}
