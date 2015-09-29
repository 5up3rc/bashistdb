package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	//names, err := net.LookupAddr("2.87.180.214")
	names, err := net.LookupAddr("147.27.199.212")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(names)
}
