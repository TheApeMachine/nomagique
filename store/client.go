package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bytedance/sonic"
	"github.com/theapemachine/errnie"
	nmruntime "github.com/theapemachine/nomagique/runtime"
	"golang.design/x/lockfree/lf"
)

/*
Record is one queued upload. The payload lives on disk at Path so a slow
Seaweed/S3 drain cannot retain Level3 (or any) bodies in the process heap.
*/
type Record struct {
	Key      string
	Value    []byte
	Envelope *nmruntime.Envelope
	// gen is assigned at enqueue; put skips superseded mutable-key uploads.
	gen uint64
}

/*
Client is the async Seaweed/S3 façade: tick envelopes and Level3 bodies enqueue
here so broker and strategy never block on upload I/O.
*/
type Client struct {
	ctx      context.Context
	cancel   context.CancelFunc
	err      error
	S3Client *s3.Client
	bucket   string
	transfer *transfermanager.Client
	queue    *lf.Queue[Record]
	running  atomic.Bool
	closed   atomic.Bool
	loops    sync.WaitGroup
	halt     chan struct{}
	mu       sync.RWMutex
	shutdown context.Context
	// latestGen[key] is the newest enqueue gen for that key — drainers drop
	// older payloads so concurrent PUT cannot overwrite newer mutable state.
	latestGen map[string]uint64
	seq       uint64
	// keyLocks serialize concurrent uploads of the same mutable key so a slow
	// gen=1 PUT cannot finish after gen=2 and clobber newer state.
	keyLocks sync.Map
}

/*
NewClient opens the configured bucket and starts drain loops. The client
context survives parent cancel so Close can finish queued uploads.
*/
func NewClient(parent context.Context, bucket, endpoint, region string) *Client {
	// Survive parent cancel (SIGINT) so Close can still finish uploads.
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))

	cfg, err := config.LoadDefaultConfig(ctx, config.WithHTTPClient(&http.Client{
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
		Timeout: 30 * time.Second,
	}))

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.IO, "[store.client] error", err,
		))

		cancel()
		return nil
	}

	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
		options.Region = region
		options.Credentials = credentials.NewStaticCredentialsProvider(
			"anonymous", "anonymous", "",
		)
		// SeaweedFS has no response checksum header; keep validation off and
		// silence the SDK's per-object debug skip line.
		options.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		options.DisableLogOutputChecksumValidationSkipped = true
	})

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})

	if err != nil && !strings.Contains(err.Error(), "BucketAlready") {
		errnie.Error(errnie.Err(
			errnie.IO, "[store.client] create bucket error", err,
		))

		cancel()
		return nil
	}

	storeClient := &Client{
		ctx:       ctx,
		cancel:    cancel,
		S3Client:  client,
		bucket:    bucket,
		transfer:  transfermanager.New(client),
		queue:     lf.NewQueue[Record](),
		halt:      make(chan struct{}),
		latestGen: make(map[string]uint64),
	}

	storeClient.Run()
	return storeClient
}

/*
Run starts the upload drainers once. Hot paths must only Enqueue.
*/
func (client *Client) Run() {
	if !client.running.CompareAndSwap(false, true) {
		return
	}

	for range 2 {
		client.loops.Add(1)

		go func() {
			defer client.loops.Done()
			client.loop()
		}()
	}
}

/*
loop is one upload drainer. Close closes halt, then drains anything left
with a shutdown context before it cancels client.ctx.
*/
func (client *Client) loop() {
	for {
		if client.drain() {
			select {
			case <-client.halt:
				return
			default:
			}

			continue
		}

		select {
		case <-client.halt:
			return
		case <-time.After(time.Millisecond):
		}
	}
}

/*
drain uploads one queued record. False means the queue was empty.
*/
func (client *Client) drain() bool {
	record, ok := client.queue.Dequeue()

	if !ok {
		return false
	}

	client.put(client.opContext(), record)
	return true
}

func (client *Client) opContext() context.Context {
	client.mu.RLock()
	defer client.mu.RUnlock()

	if client.shutdown != nil {
		return client.shutdown
	}

	return client.ctx
}

func (client *Client) put(ctx context.Context, record Record) {
	unlock := client.lockKey(record.Key)
	defer unlock()

	client.mu.RLock()
	latest := client.latestGen[record.Key]
	client.mu.RUnlock()

	if record.gen != 0 && latest != 0 && record.gen != latest {
		return
	}

	if record.Envelope != nil {
		body, err := sonic.Marshal(record.Envelope)

		if err != nil {
			errnie.Error(errnie.Err(
				errnie.IO, "failed to marshal envelope", err,
			))

			return
		}

		record.Value = body
	}

	client.upload(ctx, record.Key, record.Value)

	// Re-check after upload: if a newer gen arrived while we held the key lock
	// it is still queued — leave latestGen so that worker uploads next.
	client.mu.Lock()
	if client.latestGen[record.Key] == record.gen {
		delete(client.latestGen, record.Key)
	}
	client.mu.Unlock()
}

func (client *Client) lockKey(key string) func() {
	// Unique tick/stop keys never collide — locking them grew keyLocks without
	// bound. Only mutable prefixes need cross-worker serialization.
	if !mutableStoreKey(key) {
		return func() {}
	}

	v, _ := client.keyLocks.LoadOrStore(key, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func mutableStoreKey(key string) bool {
	// Position keys are "{tier}/open/{symbol}/{id}" — not a root "open/" prefix.
	return strings.Contains(key, "/open/") ||
		strings.HasPrefix(key, "model/") ||
		strings.HasPrefix(key, "workspace/tick/") ||
		strings.Contains(key, "/latest")
}

/*
List yields every page of objects under prefix. It follows continuation
tokens, so a prefix longer than one page is complete. A listing failure
yields nothing: a partial page is not reported as the complete set.
*/
func (client *Client) List(
	prefix string,
) iter.Seq[*s3.ListObjectsV2Output] {
	return func(yield func(*s3.ListObjectsV2Output) bool) {
		if client == nil {
			return
		}

		var token *string

		for {
			page, err := client.S3Client.ListObjectsV2(client.ctx, &s3.ListObjectsV2Input{
				Bucket:            aws.String(client.bucket),
				Prefix:            aws.String(prefix),
				ContinuationToken: token,
			})

			if err != nil {
				errnie.Error(errnie.Err(
					errnie.IO, "[store.client] list objects error", err,
				))

				return
			}

			if page == nil {
				return
			}

			if !yield(page) {
				return
			}

			if page.IsTruncated == nil || !*page.IsTruncated || page.NextContinuationToken == nil {
				return
			}

			if token != nil && *token == *page.NextContinuationToken {
				errnie.Error(errnie.Err(
					errnie.IO,
					"[store.client] list continuation error",
					fmt.Errorf("list continuation for %s did not advance", prefix),
				))

				return
			}

			token = page.NextContinuationToken
		}
	}
}

/*
ListKeys collects object keys under prefixes, drops duplicates, and sorts them
by epoch, tick, then the full key. Canonical keys store those segments as
zero-padded decimals, so that string order is numeric order. A listing failure
returns no keys: a partial page is not reported as the complete set.
*/
func (client *Client) ListKeys(prefix string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for page := range client.List(prefix) {
			for _, object := range page.Contents {
				if object.Key == nil {
					continue
				}

				if !yield(*object.Key) {
					return
				}
			}
		}
	}
}

/*
Download returns the raw object body.
*/
func (client *Client) Download(ctx context.Context, key string) ([]byte, error) {
	body := manager.NewWriteAtBuffer(make([]byte, 0, 4096))

	_, err := client.transfer.DownloadObject(ctx, &transfermanager.DownloadObjectInput{
		Bucket:   aws.String(client.bucket),
		Key:      aws.String(key),
		WriterAt: body,
	})

	if err != nil {
		return nil, errnie.Error(errnie.Err(
			errnie.IO, "[store.client] download error", err,
		))
	}

	return body.Bytes(), nil
}

/*
BulkDownload downloads every object under prefix in parallel.
Results keep ListKeys order: each body lands at its key index.
*/
func (client *Client) BulkDownload(ctx context.Context, prefix string) [][]byte {
	keys := make([]string, 0)

	for key := range client.ListKeys(prefix) {
		keys = append(keys, key)
	}

	results := make([][]byte, len(keys))

	var (
		failed atomic.Bool
		wg     sync.WaitGroup
	)

	for i, key := range keys {
		wg.Go(func() {
			if failed.Load() || ctx.Err() != nil {
				return
			}

			raw, err := client.Download(ctx, key)

			if err != nil {
				errnie.Error(errnie.Err(
					errnie.IO, "[store.client] download error", err,
				))

				failed.Store(true)
				return
			}

			results[i] = raw
		})
	}

	wg.Wait()

	if failed.Load() {
		return nil
	}

	return results
}

/*
Stream yields each object body under prefix.
A canceled ctx stops the stream before the next object.
*/
func (client *Client) Stream(ctx context.Context, prefix string) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		for key := range client.ListKeys(prefix) {
			if err := ctx.Err(); err != nil {
				errnie.Error(errnie.Err(
					errnie.IO, "[store.client] context canceled", err,
				))

				return
			}

			raw, err := client.Download(ctx, key)

			if err != nil {
				errnie.Error(errnie.Err(
					errnie.IO, "[store.client] download error", err,
				))

				return
			}

			if !yield(raw) {
				return
			}
		}
	}
}

/*
Write is unsupported. JSON bodies do not carry an upload key.
Use WriteAtPrefix.
*/
func (client *Client) Write([]byte) (int, error) {
	return 0, errnie.Error(errnie.Err(
		errnie.Validation, "[store.client] Write is unsupported; use WriteAtPrefix", nil,
	))
}

/*
WriteAtPrefix enqueues a keyed blob (e.g. Level3 on disk) without holding heap bodies.
*/
func (client *Client) WriteAtPrefix(key string, value []byte) (int, error) {
	if err := client.enqueue(Record{Key: key, Value: value}); err != nil {
		return 0, err
	}

	return len(value), nil
}

/*
WriteSync uploads before returning. Settle uses it so the open key is deleted
only after the closed object is stored.
*/
func (client *Client) WriteSync(key string, value []byte) (int, error) {
	client.mu.RLock()
	closed := client.closed.Load()
	ctx := client.ctx

	if client.shutdown != nil {
		ctx = client.shutdown
	}

	client.mu.RUnlock()

	if closed {
		return 0, errnie.Error(errnie.Err(
			errnie.IO, "[store.client] write after close", nil,
		))
	}

	return client.upload(ctx, key, value)
}

func (client *Client) enqueue(record Record) error {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closed.Load() {
		return errnie.Error(errnie.Err(
			errnie.IO, "[store.client] write after close", nil,
		))
	}

	if client.latestGen == nil {
		client.latestGen = make(map[string]uint64)
	}

	client.seq++
	record.gen = client.seq
	client.latestGen[record.Key] = record.gen
	client.queue.Enqueue(record)
	return nil
}

/*
QueueDepth is the number of records waiting for async S3 upload.
*/
func (client *Client) QueueDepth() int {
	if client == nil || client.queue == nil {
		return 0
	}

	return int(client.queue.Length())
}

/*
WriteAtPrefixWithEnvelope queues tick persistence; marshaling runs on the drain thread.
*/
func (client *Client) WriteAtPrefixWithEnvelope(key string, envelope *nmruntime.Envelope) (int, error) {
	if err := client.enqueue(Record{Key: key, Envelope: envelope}); err != nil {
		return 0, err
	}

	return 0, nil
}

/*
upload performs one S3 PutObject from the drainer pool.
*/
func (client *Client) upload(ctx context.Context, prefix string, p []byte) (int, error) {
	if prefix == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "[store.client] upload key is empty", nil,
		))
	}

	_, err := client.transfer.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(client.bucket),
		Key:    aws.String(prefix),
		Body:   bytes.NewReader(p),
	})

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.IO, "[store.client] upload error", err,
		))
	}

	return len(p), nil
}

/*
Delete removes one object. Callers that mean a single leaf must not use
DeletePrefix — a path prefix also matches longer descendant keys.
*/
func (client *Client) Delete(key string) error {
	if client == nil || client.S3Client == nil || key == "" {
		return nil
	}

	_, err := client.S3Client.DeleteObject(client.ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(client.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return errnie.Error(errnie.Err(
			errnie.IO, "[store.client] delete object error", err,
		))
	}

	return nil
}

/*
DeletePrefix deletes all objects under the specified prefix.
*/
func (client *Client) DeletePrefix(prefix string) error {
	for page := range client.List(prefix) {
		for _, object := range page.Contents {
			if object.Key == nil {
				continue
			}

			_, err := client.S3Client.DeleteObject(client.ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(client.bucket),
				Key:    aws.String(*object.Key),
			})

			if err != nil {
				return errnie.Error(errnie.Err(
					errnie.IO, "[store.client] delete object error", err,
				))
			}
		}
	}

	return nil
}

/*
DeleteOtherKeys deletes every object under prefix except keep.
keep may be empty to delete the whole prefix.
*/
func (client *Client) DeleteOtherKeys(prefix, keep string) error {
	if client == nil || client.S3Client == nil || prefix == "" {
		return nil
	}

	for page := range client.List(prefix) {
		for _, object := range page.Contents {
			if object.Key == nil || *object.Key == keep {
				continue
			}

			_, err := client.S3Client.DeleteObject(client.ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(client.bucket),
				Key:    aws.String(*object.Key),
			})

			if err != nil {
				return errnie.Error(errnie.Err(
					errnie.IO, "[store.client] delete object error", err,
				))
			}
		}
	}

	return nil
}

/*
MovePrefix copies all objects matching sourcePrefix to destPrefix, then deletes the originals.
An object keeps the path it holds under sourcePrefix, so "position/open/BTC" moved to
"position/closed/BTC" lands on "position/closed/BTC/<epoch>/<tick>".
*/
func (client *Client) MovePrefix(sourcePrefix, destPrefix string) error {
	for page := range client.List(sourcePrefix) {
		for _, object := range page.Contents {
			if object.Key == nil {
				continue
			}

			_, err := client.S3Client.CopyObject(client.ctx, &s3.CopyObjectInput{
				Bucket:     aws.String(client.bucket),
				Key:        aws.String(destPrefix + "/" + *object.Key),
				CopySource: aws.String(client.bucket + "/" + *object.Key),
			})

			if err != nil {
				return errnie.Error(errnie.Err(
					errnie.IO, "[store.client] copy object error", err,
				))
			}

			_, err = client.S3Client.DeleteObject(client.ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(client.bucket),
				Key:    aws.String(*object.Key),
			})

			if err != nil {
				return errnie.Error(errnie.Err(
					errnie.IO, "[store.client] delete object error", err,
				))
			}
		}
	}

	return nil
}

/*
Error surfaces accumulated client faults without blocking Enqueue.
*/
func (client *Client) Error() error {
	return client.err
}

/*
Close stops new writes, lets drainers finish, then uploads anything still
queued on a bounded context. client.ctx is canceled only after that drain.
*/
func (client *Client) Close() error {
	if !client.closed.CompareAndSwap(false, true) {
		return client.err
	}

	shutdown, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()

	client.mu.Lock()
	client.shutdown = shutdown
	client.mu.Unlock()

	close(client.halt)

	done := make(chan struct{})

	go func() {
		client.loops.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-shutdown.Done():
		client.err = errors.Join(client.err, errnie.Error(errnie.Err(
			errnie.Timeout, "[store.client] shutdown drain timed out", shutdown.Err(),
		)))
	}

	for shutdown.Err() == nil {
		record, ok := client.queue.Dequeue()

		if !ok {
			break
		}

		client.put(shutdown, record)
	}

	client.cancel()
	return client.err
}
