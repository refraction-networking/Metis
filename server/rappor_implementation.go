package main

import (
	"fmt"
	"crypto/rand"
)

/*
RAPPOR encoding parameters.
These affect privacy/anonymity. See paper for details.
 */
type Params struct{
	//k
	numBloomBits int
	//h
	numHashes int
	//m
	numCohorts int
	//p
	probP float64
	//q
	probQ float64
	//f
	probF float64
}

func (p Params) init(){
	p.numBloomBits = 16
	p.numHashes = 2
	p.numCohorts = 64
	p.probP = 0.50
	p.probQ = 0.75
	p.probF = 0.50
}

type SecureRandom struct {
	probOne float64
	numBits int
}

func (s *SecureRandom) init(probOne float64, numBits int) {
	s.probOne = probOne
	s.numBits = numBits
}

func (s *SecureRandom) call() {
	p := s.probOne
	r := 0
	b := make([]byte, s.numBits)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	var i uint
	for i = 0; i < uint(s.numBits); i++ {
		fmt.Println("b[i]: ", uint(b[i]), ", p*256: ", uint(p*256))
		if uint(b[i]) < uint(p*256) {
			r |= 0x1 << i
		}
		fmt.Printf("r: %04b\n", r)
	}

}

func main() {
	var s SecureRandom
	s.init(0.5,4)
	s.call()
}