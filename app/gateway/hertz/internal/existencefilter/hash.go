package existencefilter

import (
	"strconv"

	"github.com/cespare/xxhash/v2"
)

const hashAlgorithm = "xxhash64-double-v1"

func bitPositions(productID int64, bitCount, hashCount uint64) []int64 {
	value := strconv.FormatInt(productID, 10)
	first := xxhash.Sum64String("flashmall:product:first:" + value)
	second := xxhash.Sum64String("flashmall:product:second:" + value)
	if second == 0 {
		second = 0x9e3779b97f4a7c15
	}
	positions := make([]int64, 0, hashCount)
	for index := uint64(0); index < hashCount; index++ {
		positions = append(positions, int64((first+index*second)%bitCount))
	}
	return positions
}
