package redispub

import (
	"strconv"
	"strings"
	"time"
)

// streamIDSeparator splits a stream id into its millisecond and sequence parts.
const streamIDSeparator = "-"

// OlderStreamID reports whether the left stream id precedes the right one. Ids
// have the form "<milliseconds>-<sequence>" and both parts have to be read as
// numbers: "9999999999999-0" sorts after "10000000000000-0" as a string.
func OlderStreamID(left, right string) bool {
	leftMillis, leftSeq := parseStreamID(left)
	rightMillis, rightSeq := parseStreamID(right)

	if leftMillis != rightMillis {
		return leftMillis < rightMillis
	}

	return leftSeq < rightSeq
}

// StreamIDAt is the lowest stream id an entry created at the given instant can
// carry. Redis derives ids from the wall clock, so an age-based retention bound
// is expressed as the id at the cutoff rather than as separate bookkeeping.
func StreamIDAt(instant time.Time) string {
	return strconv.FormatInt(instant.UnixMilli(), 10) + streamIDSeparator + "0"
}

func parseStreamID(id string) (int64, int64) {
	millisPart, seqPart, _ := strings.Cut(id, streamIDSeparator)

	millis, _ := strconv.ParseInt(millisPart, 10, 64)
	seq, _ := strconv.ParseInt(seqPart, 10, 64)

	return millis, seq
}
