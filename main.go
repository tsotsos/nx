package main

import (
	"fmt"
	"github.com/joho/godotenv"
)

func init() {
	sessionId = ""
	if err := godotenv.Load(); err != nil {
		fmt.Println(err)
	}
}

func main() {
	conf := NewConfig()
	ZoneNames(conf)
	st, err := ZonesStatuses(conf)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(st)
}
