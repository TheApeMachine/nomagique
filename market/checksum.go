package market

import (
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/shopspring/decimal"
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
checksumToken formats a book decimal the way Kraken's CRC expects: fixed to the
value's stored scale (pair/lot decimals), then strip "." and leading zeros.
Decimal.String() drops trailing zeros ("2641.63120" → "2641.6312") and breaks
the venue checksum; StringFixed keeps them.
*/
func checksumToken(value *decimal.Decimal) string {
	if value == nil {
		return ""
	}

	places := int32(0)

	if exp := value.Exponent(); exp < 0 {
		places = -exp
	}

	return trimChecksum(value.StringFixed(places))
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

		part := checksumToken(cursor.Price) + checksumToken(cursor.Quantity)
		result.Asks += part
		cursor = cursor.Higher
	}

	cursor = book.BestBid()

	for range 10 {
		if cursor == nil {
			break
		}

		part := checksumToken(cursor.Price) + checksumToken(cursor.Quantity)
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
			part := checksumToken(order.LimitPrice) + checksumToken(order.Quantity)
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
			part := checksumToken(order.LimitPrice) + checksumToken(order.Quantity)
			result.Bids += part
		}

		cursor = cursor.Lower
	}

	result.LocalChecksum = fmt.Sprint(crc32.ChecksumIEEE([]byte(result.Asks + result.Bids)))
	result.Match = result.LocalChecksum == result.ServerChecksum
	return result
}
