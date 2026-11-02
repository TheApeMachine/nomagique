package store

import (
	"context"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.design/x/lockfree/lf"
)

func TestClientRefusesEmptyUploadKey(t *testing.T) {
	Convey("WriteAtPrefix and Write refuse an empty key", t, func() {
		client := &Client{
			ctx:    context.Background(),
			bucket: "test-bucket",
		}

		n, err := client.WriteAtPrefix("", []byte("payload"))
		So(err, ShouldNotBeNil)
		So(n, ShouldEqual, 0)

		n, err = client.Write([]byte("payload"))
		So(err, ShouldNotBeNil)
		So(n, ShouldEqual, 0)
	})
}

func TestWriteAtPrefixSpoolsPayload(t *testing.T) {
	Convey("WriteAtPrefix keeps the payload on disk, not in the queue record", t, func() {
		dir := t.TempDir()
		client := &Client{
			ctx:      context.Background(),
			bucket:   "test-bucket",
			queue:    lf.NewQueue[Record](),
			spoolDir: dir,
		}

		n, err := client.WriteAtPrefix("market/level3/part.bin", []byte("frame-bytes"))
		So(err, ShouldBeNil)
		So(n, ShouldEqual, len("frame-bytes"))

		record, ok := client.queue.Dequeue()
		So(ok, ShouldBeTrue)
		So(record.Key, ShouldEqual, "market/level3/part.bin")
		So(record.Path, ShouldNotBeEmpty)

		body, err := os.ReadFile(record.Path)
		So(err, ShouldBeNil)
		So(string(body), ShouldEqual, "frame-bytes")
	})
}

func TestDestKeyKeepsPathUnderSourcePrefix(t *testing.T) {
	Convey("destKey keeps the path under the moved prefix", t, func() {
		cases := []struct {
			source string
			dest   string
			key    string
			want   string
		}{
			{
				source: "position/open/BTC/USD",
				dest:   "position/closed/BTC/USD",
				key:    "position/open/BTC/USD/1792668320433768000/0000000000000000230",
				want:   "position/closed/BTC/USD/1792668320433768000/0000000000000000230",
			},
			{
				source: "position/open/BTC/USD/",
				dest:   "position/closed/BTC/USD/",
				key:    "position/open/BTC/USD/1792668320433768000/0000000000000000230",
				want:   "position/closed/BTC/USD/1792668320433768000/0000000000000000230",
			},
			{
				source: "position/open/BTC/USD/1792668320433768000/0000000000000000230",
				dest:   "position/closed/BTC/USD/1792668320433768000/0000000000000000230",
				key:    "position/open/BTC/USD/1792668320433768000/0000000000000000230",
				want:   "position/closed/BTC/USD/1792668320433768000/0000000000000000230",
			},
		}

		for _, test := range cases {
			got := destKey(test.dest, test.key, test.source)
			So(got, ShouldEqual, test.want)
			So(got, ShouldNotEqual, test.key)
		}
	})
}
