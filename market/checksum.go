package market

import (
	"fmt"
	"hash/crc32"
	"strings"
)

/*
ChecksumResult is the outcome of an L2/L3 book checksum.
*/
type ChecksumResult struct {
	ServerChecksum string
	LocalChecksum  string
	Match          bool
	Asks           string
	Bids           string
}

func trimChecksum(value string) string {
	return strings.TrimLeft(strings.ReplaceAll(value, ".", ""), "0")
}

/*
L2Checksum verifies top-of-book L2 state against the venue checksum.
*/
func (book *Book) L2Checksum(checksum string) *ChecksumResult {
	result := &ChecksumResult{ServerChecksum: checksum}
	cursor := book.BestAsk()

	for range 10 {
		if cursor == nil {
			break
		}

		part := trimChecksum(cursor.Price.String()) + trimChecksum(cursor.Quantity.String())
		result.Asks += part
		cursor = cursor.Higher
	}

	cursor = book.BestBid()

	for range 10 {
		if cursor == nil {
			break
		}

		part := trimChecksum(cursor.Price.String()) + trimChecksum(cursor.Quantity.String())
		result.Bids += part
		cursor = cursor.Lower
	}

	result.LocalChecksum = fmt.Sprint(crc32.ChecksumIEEE([]byte(result.Asks + result.Bids)))
	result.Match = result.LocalChecksum == result.ServerChecksum
	return result
}

/*
L3Checksum verifies top-of-book L3 order queues against the venue checksum.
*/
func (book *Book) L3Checksum(checksum string) *ChecksumResult {
	result := &ChecksumResult{ServerChecksum: checksum}
	cursor := book.BestAsk()

	for range 10 {
		if cursor == nil {
			break
		}

		for _, order := range cursor.Queue() {
			part := trimChecksum(order.LimitPrice.String()) + trimChecksum(order.Quantity.String())
			result.Asks += part
		}

		cursor = cursor.Higher
	}

	cursor = book.BestBid()

	for range 10 {
		if cursor == nil {
			break
		}

		for _, order := range cursor.Queue() {
			part := trimChecksum(order.LimitPrice.String()) + trimChecksum(order.Quantity.String())
			result.Bids += part
		}

		cursor = cursor.Lower
	}

	result.LocalChecksum = fmt.Sprint(crc32.ChecksumIEEE([]byte(result.Asks + result.Bids)))
	result.Match = result.LocalChecksum == result.ServerChecksum
	return result
}
