package main

import (
	"fmt"
	//"github.com/davecgh/go-spew/spew"
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
	/*	names, err := ZonesNames(conf)
		if err != nil {
			panic(err)
		}
		st, err := ZonesStatuses(conf)
		if err != nil {
			panic(err)
		}
		for i, v := range st {
			fmt.Println(names[i])
			fmt.Println(v)
		}
		//overal, _ := Status(conf)
		//spew.Dump(overal)*/
	err := SetByPass(1, conf)
	if err != nil {
		panic(err)
	}
}
