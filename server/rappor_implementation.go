package main

import (
	"fmt"
	"crypto/rand"
	"encoding/binary"
	"crypto/md5"
	"io"
	"crypto/hmac"
	"crypto/sha256"
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

func (p *Params) init(){
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

func (s *SecureRandom) setBits() (int, error) {
	p := s.probOne
	r := 0
	b := make([]byte, s.numBits)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("error:", err)
		return 0, err
	}
	var i uint
	for i = 0; i < uint(s.numBits); i++ {
		fmt.Println("b[i]: ", uint(b[i]), ", p*256: ", uint(p*256))
		if uint(b[i]) < uint(p*256) {
			r |= 0x1 << i
		}
		fmt.Printf("r: %04b\n", r)
	}
	return r, nil

}

func (s *SecureRandom) init(probOne float64, numBits int) (int, error){
	s.probOne = probOne
	s.numBits = numBits
	r, err := s.setBits()
	if err != nil {
		return 0, err
	}
	fmt.Println(r, err)
	return r, nil
}

type SecureIrrRand struct {
	pGen int
	qGen int
	numBits int
}

func (s *SecureIrrRand) init(params Params) {
	s.numBits = params.numBloomBits
	var sr SecureRandom
	pGen, err := sr.init(params.probP, s.numBits)
	if err != nil {
		fmt.Println("Error generating pGen")
	} else {
		s.pGen = pGen
	}
	qGen, err := sr.init(params.probQ, s.numBits)
	if err != nil {
		fmt.Println("Error generating qGen")
	} else {
		s.qGen = qGen
	}
}

func toBigEndian(i int64) []byte {
	/*Convert integer to 4-byte big endian string*/
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(buf, i)
	fmt.Printf("%x\n", buf[:n])
	return buf
}

func getBloomBits(word []byte, cohort int, numHashes int, numBloombits int) []int {
	/*
	Return an array of bits to set in the bloom filter.
	In the real report, we bitwise-OR them together.  In hash candidates, we put
	them in separate entries in the "map" matrix.
	*/
	//Cohort is 4 byte prefix
	//Need variadic ellipsis because word is []byte, not byte
	value := append(toBigEndian(int64(cohort)), word...)
	h := md5.New()
	io.WriteString(h, string(value))
	digest := h.Sum(nil)

	// Each hash is a byte, which means we could have up to 256 bit Bloom filters.
	// There are 16 bytes in an MD5, in which case we can have up to 16 hash
	// functions per Bloom filter.
	if numHashes > len(digest) {
		fmt.Println("Error: can't have more than ", len(digest), " hashes")
		return []int{}
	}
	var retVal []int
	for i := 0; i < numHashes; i++ {
		retVal = append(retVal, int(digest[i])%numBloombits)
	}
	return retVal
}

func getPrrMasks(secret string, word string, probF float64) (int, int){
	key := []byte(secret)
	hash := hmac.New(sha256.New, key)
	hash.Write([]byte(word))
	digestBytes := hash.Sum(nil)
	if len(digestBytes) != 32 {
		panic("digest_bytes needs to be 32 bytes long!")
	}
	threshold128 := probF*128
	uniform := 0
	fMask := 0

	for i, ch := range digestBytes {
		byt := int(ch)
		uBit := byt & 0x01 //1 bit of entropy
		uniform |= uBit << uint(i) //maybe set bit in mask
		rand128 := byt >> 1 //7 bits of entropy
		var noiseBit int
		if float64(rand128) < threshold128 {
			noiseBit = 0x01
		} else {
			noiseBit = 0x00
		}
		fMask |= noiseBit << uint(i)
	}
	return uniform, fMask
}

type Encoder struct{
	params Params
	cohort int
	secret string
	irrRand *SecureIrrRand
}

func (e *Encoder) init(params Params, cohort int, secret string, irrRand *SecureIrrRand) {
	/*
	Args:
		params: RAPPOR Params() controlling privacy
		cohort: integer cohort, for Bloom hashing.
		secret: secret string, for the PRR to be a deterministic function of the
	reported value.
		irr_rand: IRR randomness interface.
	*/
	// RAPPOR params.  NOTE: num_cohorts isn't used.  p and q are used by irr_rand.
	e.params = params
	e.cohort = cohort
	e.secret = secret
	e.irrRand = irrRand
}

func (e *Encoder) internalEncodeBits(bits int) (int, int){
	/*
	Helper function for simulation / testing.
	Returns:
		The PRR and IRR.  The PRR should never be sent over the network.
	*/
	// Compute Permanent Randomized Response (PRR). Uniform and fMask are 32 bits long.
	uniform, fMask := getPrrMasks(e.secret, string(toBigEndian(int64(bits))), e.params.probF)
	/*
	Suppose bit i of the Bloom filter is B_i.  Then bit i of the PRR is
	defined as:
		1   with prob f/2
		0   with prob f/2
		B_i with prob 1-f

	Uniform bits are 1 with probability 1/2, and f_mask bits are 1 with
	probability f.  So in the expression below:

	- Bits in (uniform & f_mask) are 1 with probability f/2.
	- (bloom_bits & ~f_mask) clears a bloom filter bit with probability
	f, so we get B_i with probability 1-f.
	- The remaining bits are 0, with remaining probability f/2.
	*/
	//TODO: Make certain that ^x in Go === ~x in Python
	prr := (int(bits) & ^fMask) | (uniform & fMask)

	// Compute Instantaneous Randomized Response (IRR).
	// If PRR bit is 0, IRR bit is 1 with probability p.
	// If PRR bit is 1, IRR bit is 1 with probability q.
	e.irrRand.init(e.params)
	pBits := e.irrRand.pGen
	qBits := e.irrRand.qGen

	irr := (pBits & ^prr) | (qBits & prr)

	return prr, irr  // IRR is the rappor
}

func (e *Encoder) internalEncode(word []byte) (int, int, int) {
	/*
	Helper function for simulation / testing.
	Returns:
		The Bloom filter bits, PRR, and IRR.  The first two values should never
		be sent over the network.
	*/
	bloomBits := getBloomBits(word, e.cohort, e.params.numHashes, e.params.numBloomBits)
	bloom := 0
	for bitToSet := range bloomBits {
		bloom |= 1 << uint(bitToSet)
	}
	prr, irr := e.internalEncodeBits(bloom)
	return bloom, prr, irr
}

func (e *Encoder) encodeBits(bits int) int {
	/*
	Encode a string with RAPPOR.
	Args:
		bits: An integer representing bits to encode.
	Returns:
		An integer that is the IRR (Instantaneous Randomized Response).
	*/
	_, irr := e.internalEncodeBits(bits)
	return irr
}

func (e *Encoder) encode(word []byte) int {
	/*Encode a string with RAPPOR.
Args:
word: the string that should be privately transmitted.
		Returns:
	An integer that is the IRR (Instantaneous Randomized Response).
	*/
	_, _, irr := e.internalEncode(word)
	return irr
}

func estimateSetBits(reports []int, params Params) []float64 {
	/*
	Estimate which bits were truly set in B for a particular cohort.
	Returns: Y, a slice of the number of times each bit was estimated
	to have been truly set in B.
	This function will be called for each cohort. j represents the jth
	cohort and is included only to keep variable names matching the paper.
	 */
	p := params.probP
	q := params.probQ
	f := params.probF
	Nj := float64(len(reports))
	var Y_j []float64
	for i:=0; i<32; i++ {
		c_ij := 0.0
		for _, rep := range reports {
			if rep & (1<<uint(i)) == 1 {
				c_ij += 1.0
			}
		}
		t_ij := c_ij-(p+0.5*f*q-0.5*f*p)*Nj/((1-f)*(q-p))
		Y_j = append(Y_j, t_ij)
	}
	return Y_j
}

func main() {
	/*var s SecureRandom
	s.init(0.5,4)
	s.setBits()*/
	var s SecureIrrRand
	var p Params
	p.init()
	s.init(p)
	getBloomBits([]byte("asdf"), 0, 0, 0)
	var e Encoder
	e.init(p, 1,"my secret string is very secret", &s)
	rep1 := e.encode([]byte("google.com"))
	rep2 := e.encode([]byte("facebook.com"))
	fmt.Println(rep1, ", ", rep2)
}