package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	names, err := net.LookupAddr("2.87.180.214")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(names)
}
