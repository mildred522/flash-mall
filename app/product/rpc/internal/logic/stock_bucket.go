package logic

import "hash/crc32"

func stockBucketCount(configured int) int {
	if configured <= 0 {
		return 1
	}
	return configured
}

func stockBucketIndex(orderId string, bucketCount int) int {
	if bucketCount <= 1 {
		return 0
	}
	hash := crc32.ChecksumIEEE([]byte(orderId))
	return int(hash % uint32(bucketCount))
}
