package main

type Filter struct {
	bitArray []uint8
}

func (f Filter) init(size int) {
	if size <=0 {size = 1}
	for i := 0; i < size; i++ {
		f.bitArray[i] = 0
	}
}

func (f Filter) setBit(idx uint) {
	byteIdx := idx/8
	bitIdx := idx%8
	f.bitArray[byteIdx] = f.bitArray[byteIdx] | (1<<bitIdx)
}

type BloomFilter struct {
	size uint
	numHashes uint
	filter Filter
}

func (b BloomFilter) addString(s string) {

}
