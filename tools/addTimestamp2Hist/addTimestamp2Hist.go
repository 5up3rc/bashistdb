/*
Command addTimestamp2Hist adds timestamps to an untimestamped
bash history file. You may choose the starting date as X months
before now. Then the utility will add timestamps at equal
intervals for every line in history.

    $ addTimestamp2Hist ~/.bash_history
*/
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

var home = os.Getenv("HOME")

var (
	historyFile = flag.String("f", home+"/.bash_history", "file to process")
	duration    = flag.Int("since", 3, "span timestamps in equal spaces between N months and now")
)

func main() {
	flag.Parse()

	historyIn, err := ioutil.ReadFile(*historyFile)
	if err != nil {
		log.Fatalln(err)
	}

	hasTimestamp := regexp.MustCompile("^#")
	lines := strings.Split(string(historyIn), "\n")
	now := time.Now()
	since := now.AddDate(0, -1**duration, 0)
	// since := time.Now().Add(-1 * time.Duration(*duration) * 30 * 24 * time.Hour)
	space := now.Sub(since) / time.Duration(len(lines))
	for i := 0; i < len(lines); i++ {
		if hasTimestamp.MatchString(lines[i]) {
			fmt.Printf("%s\n%s\n", lines[i], lines[i+1])
			i++
			continue
		}
		if lines[i] != "" {
			fmt.Printf("#%d\n", since.Unix()+int64(i)*int64(space.Seconds()))
			fmt.Printf("%s\n", lines[i])
		}
	}
}
