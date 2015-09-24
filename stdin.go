package main

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"time"
)

func readFromStdin(r *bufio.Reader) error {
	//                                  LINENUM        DATETIME         CM
	parseLine := regexp.MustCompile(`^ *[0-9]+\*? *([0-9T:+-]{24,24}) *(.*)`)
	for {
		historyLine, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			} else {
				return err
			}
		}
		args := parseLine.FindStringSubmatch(historyLine)
		if len(args) != 3 {
			info.Println("Could't decode line. Skipping:", historyLine)
			continue
		}
		time, err := time.Parse(RFC3339alt, args[1])
		if err != nil {
			return err
		}
		err = submitRecord(*user, *hostname, strings.TrimSuffix(args[2], "\n"), time)
		if err != nil {
			return err
		}
	}
}
