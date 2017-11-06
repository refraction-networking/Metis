package main

//TODO: What does Metis's license file have to look like if I include this?

import(
	"github.com/willf/bloom"
)

var filter *bloom.BloomFilter

func initFilter(){
	//TODO: figure out how to size the filter correctly. Error rate?
	n := uint(1000)
	filter = bloom.New(20*n, 5)
}

func addStr(str string){
	filter.Add([]byte(str))
}

func testStr(str string) bool {
	return filter.Test([]byte(str))
}
