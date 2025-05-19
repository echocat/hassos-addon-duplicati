package main

import (
	"fmt"
	"strconv"
)

func q(in string) string {
	return strconv.Quote(in)
}

func recordmln(k, v string) string {
	return k + "<<EOF\n" + v + "EOF\n"
}

func recordln(k, v string, args ...any) string {
	return k + "=" + strconv.Quote(fmt.Sprintf(v, args...)) + "\n"
}
