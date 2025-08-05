package main

import (
	"fmt"

	"github.com/Mister-dev-oss/ShardedPostgre/sharding"
)

func main() {
	_, err := sharding.InitRingManager("sharding/config.json")
	if err != nil {
		fmt.Println(err)
		return
	}

}
