package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"runtime"
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
	"github.com/panjf2000/ants/v2"
	"github.com/theapemachine/errnie"
	"golang.design/x/lockfree/lf"
)

/*
Record is one queued upload. The payload lives on disk at Path so a slow
Seaweed/S3 drain cannot retain Level3 (or any) bodies in the process heap.
*/
type Record struct {
	Key   string
	Value []byte
}

type Client struct {
	ctx          context.Context
	cancel       context.CancelFunc
	err          error
	S3Client     *s3.Client
	bucket       string
	transfer     *transfermanager.Client
	queue        *lf.Queue[Record]
	downloadPool *ants.Pool
	uploadPool   *ants.Pool
	running      bool
	closed       atomic.Bool
	uploads      sync.WaitGroup
}

func NewClient(parent context.Context, bucket string) *Client {
	// Survive parent cancel (SIGINT) so Flush can still finish uploads.
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))

	downloadPool, err := ants.NewPool(
		runtime.GOMAXPROCS(0),
		ants.WithNonblocking(true),
	)

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.IO, "[store.client] download pool error", err,
		))

		cancel()
		return nil
	}

	// I/O-bound uploads: larger than GOMAXPROCS, blocking so Run waits
	// for a worker instead of spinning on ErrPoolOverload.
	uploadPool, err := ants.NewPool(runtime.GOMAXPROCS(0) * 8)

	if err != nil {
		downloadPool.Release()
		errnie.Error(errnie.Err(
			errnie.IO, "[store.client] upload pool error", err,
		))

		cancel()
		return nil
	}

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
		downloadPool.Release()
		uploadPool.Release()
		errnie.Error(errnie.Err(
			errnie.IO, "[store.client] error", err,
		))

		cancel()
		return nil
	}

	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String("http://s3.seaweed.home.arpa")
		options.UsePathStyle = true
		options.Region = "us-east-1"
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
		downloadPool.Release()
		uploadPool.Release()
		errnie.Error(errnie.Err(
			errnie.IO, "[store.client] create bucket error", err,
		))

		cancel()
		return nil
	}

	storeClient := &Client{
		ctx:          ctx,
		cancel:       cancel,
		S3Client:     client,
		bucket:       bucket,
		transfer:     transfermanager.New(client),
		queue:        lf.NewQueue[Record](),
		downloadPool: downloadPool,
		uploadPool:   uploadPool,
	}

	return storeClient
}

func (client *Client) Run() {
	if client.running {
		return
	}

	client.running = true

	for client.queue.Length() > 0 {
		client.uploads.Add(1)

		task := func() {
			defer client.uploads.Done()

			record, ok := client.queue.Dequeue()

			if !ok {
				return
			}

			client.upload(client.ctx, record.Key, record.Value)
		}

		if err := client.uploadPool.Submit(task); err != nil {
			client.uploads.Done()
			errnie.Error(errnie.Err(
				errnie.IO, "[store.client] submit task error", err,
			))

			break
		}
	}

	client.running = false
}

/*
Flush submits every queued upload and waits until they finish.
*/
func (client *Client) Flush() {
	for {
		client.Run()

		if client.queue.Length() == 0 {
			break
		}

		if client.uploadPool.IsClosed() {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	client.uploads.Wait()
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
		wg.Add(1)

		if err := client.downloadPool.Submit(func() {
			defer wg.Done()

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
		}); err != nil {
			errnie.Error(errnie.Err(
				errnie.IO, "[store.client] submit task error", err,
			))

			return nil
		}
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

func (client *Client) WriteAtPrefix(key string, value []byte) (int, error) {
	if client.closed.Load() {
		return 0, errnie.Error(errnie.Err(
			errnie.IO, "[store.client] write after close", nil,
		))
	}

	client.queue.Enqueue(Record{Key: key, Value: value})
	client.Run()
	return len(value), nil
}

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

func (client *Client) Error() error {
	return client.err
}

func (client *Client) Close() error {
	if !client.closed.CompareAndSwap(false, true) {
		return client.err
	}

	client.Flush()
	client.cancel()

	var errs error

	if err := client.uploadPool.ReleaseTimeout(30 * time.Second); err != nil {
		errs = errors.Join(errs, errnie.Error(errnie.Err(
			errnie.IO, "[store.client] upload pool release", err,
		)))
	}

	if err := client.downloadPool.ReleaseTimeout(5 * time.Second); err != nil {
		errs = errors.Join(errs, errnie.Error(errnie.Err(
			errnie.IO, "[store.client] download pool release", err,
		)))
	}

	client.err = errors.Join(client.err, errs)
	return client.err
}
