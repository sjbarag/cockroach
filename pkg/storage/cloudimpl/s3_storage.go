// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package cloudimpl

import (
	"context"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/storage/cloud"
	"github.com/cockroachdb/cockroach/pkg/util/contextutil"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/tracing"
	"github.com/cockroachdb/errors"
	"github.com/opentracing/opentracing-go"
)

type s3Storage struct {
	bucket   *string
	conf     *roachpb.ExternalStorage_S3
	ioConf   base.ExternalIODirConfig
	settings *cluster.Settings
	prefix   string

	opts   s3ClientConfig
	cached *s3Client
}

// s3Client wraps an SDK client.
type s3Client struct {
	client *s3.S3
}

var reuseSession = settings.RegisterBoolSetting(
	"cloudstorage.s3.session_reuse.enabled",
	"persist the last opened s3 session and re-use it when opening a new session with the same arguments",
	true,
)

// s3ClientConfig is the immutable config used to initialize an s3 session.
// It contains values copied from corresponding fields in ExternalStorage_S3
// which are used by the session (but not those that are only used by individual
// requests).
type s3ClientConfig struct {
	// copied from ExternalStorage_S3.
	endpoint, region, bucket, accessKey, secret, tempToken, auth string
	// log.V(2) decides session init params so include it in key.
	verbose bool
}

func clientConfig(conf *roachpb.ExternalStorage_S3) s3ClientConfig {
	return s3ClientConfig{
		endpoint:  conf.Endpoint,
		region:    conf.Region,
		bucket:    conf.Bucket,
		accessKey: conf.AccessKey,
		secret:    conf.Secret,
		tempToken: conf.TempToken,
		auth:      conf.Auth,
		verbose:   log.V(2),
	}
}

var s3ClientCache struct {
	syncutil.Mutex
	// TODO(dt): make this an >1 item cache e.g. add a FIFO ring.
	key    s3ClientConfig
	client *s3Client
}

var _ cloud.ExternalStorage = &s3Storage{}

type serverSideEncMode string

const (
	kmsEnc    serverSideEncMode = "aws:kms"
	aes256Enc serverSideEncMode = "AES256"
)

func s3QueryParams(conf *roachpb.ExternalStorage_S3) string {
	q := make(url.Values)
	setIf := func(key, value string) {
		if value != "" {
			q.Set(key, value)
		}
	}
	setIf(AWSAccessKeyParam, conf.AccessKey)
	setIf(AWSSecretParam, conf.Secret)
	setIf(AWSTempTokenParam, conf.TempToken)
	setIf(AWSEndpointParam, conf.Endpoint)
	setIf(S3RegionParam, conf.Region)
	setIf(AuthParam, conf.Auth)
	setIf(AWSServerSideEncryptionMode, conf.ServerEncMode)
	setIf(AWSServerSideEncryptionKMSID, conf.ServerKMSID)

	return q.Encode()
}

// MakeS3Storage returns an instance of S3 ExternalStorage.
func MakeS3Storage(
	ctx context.Context,
	ioConf base.ExternalIODirConfig,
	conf *roachpb.ExternalStorage_S3,
	settings *cluster.Settings,
) (cloud.ExternalStorage, error) {
	if conf == nil {
		return nil, errors.Errorf("s3 upload requested but info missing")
	}

	if conf.Endpoint != "" {
		if ioConf.DisableHTTP {
			return nil, errors.New(
				"custom endpoints disallowed for s3 due to --external-io-disable-http flag")
		}
	}

	switch conf.Auth {
	case "", AuthParamSpecified:
		if conf.AccessKey == "" {
			return nil, errors.Errorf(
				"%s is set to '%s', but %s is not set",
				AuthParam,
				AuthParamSpecified,
				AWSAccessKeyParam,
			)
		}
		if conf.Secret == "" {
			return nil, errors.Errorf(
				"%s is set to '%s', but %s is not set",
				AuthParam,
				AuthParamSpecified,
				AWSSecretParam,
			)
		}
	case AuthParamImplicit:
		if ioConf.DisableImplicitCredentials {
			return nil, errors.New(
				"implicit credentials disallowed for s3 due to --external-io-implicit-credentials flag")
		}
	default:
		return nil, errors.Errorf("unsupported value %s for %s", conf.Auth, AuthParam)
	}

	// Ensure that a KMS ID is specified if server side encryption is set to use
	// KMS.
	if conf.ServerEncMode != "" {
		switch conf.ServerEncMode {
		case string(aes256Enc):
		case string(kmsEnc):
			if conf.ServerKMSID == "" {
				return nil, errors.New("AWS_SERVER_KMS_ID param must be set" +
					" when using aws:kms server side encryption mode.")
			}
		default:
			return nil, errors.Newf("unsupported server encryption mode %s. "+
				"Supported values are `aws:kms` and `AES256`.", conf.ServerEncMode)
		}
	}

	s := &s3Storage{
		bucket:   aws.String(conf.Bucket),
		conf:     conf,
		ioConf:   ioConf,
		prefix:   conf.Prefix,
		settings: settings,
		opts:     clientConfig(conf),
	}

	reuse := reuseSession.Get(&settings.SV)
	if !reuse {
		return s, nil
	}

	s3ClientCache.Lock()
	defer s3ClientCache.Unlock()

	if s3ClientCache.key == s.opts {
		s.cached = s3ClientCache.client
		return s, nil
	}

	// Make the client and cache it *while holding the lock*. We want to keep
	// other callers from making clients in the meantime, not just to avoid making
	// duplicate clients in a race but also because making clients concurrently
	// can fail if the AWS metadata server hits its rate limit.
	client, _, err := newClient(ctx, s.opts, s.settings)
	if err != nil {
		return nil, err
	}
	s.cached = &client
	s3ClientCache.key = s.opts
	s3ClientCache.client = &client
	return s, nil
}

// newClient creates a client from the passed s3ClientConfig and if the passed
// config's region is empty, used the passed bucket to determine a region and
// configures the client with it as well as returning it (so the caller can
// remember it for future calls).
func newClient(
	ctx context.Context, conf s3ClientConfig, settings *cluster.Settings,
) (s3Client, string, error) {

	// Open a span if client creation will do IO/RPCs to find creds/bucket region.
	if conf.region == "" || conf.auth == AuthParamImplicit {
		var sp opentracing.Span
		ctx, sp = tracing.ChildSpan(ctx, "open s3 client")
		defer tracing.FinishSpan(sp)
	}

	opts := session.Options{}

	if conf.endpoint != "" {
		opts.Config.Endpoint = aws.String(conf.endpoint)
		opts.Config.S3ForcePathStyle = aws.Bool(true)

		if conf.region == "" {
			conf.region = "default-region"
		}

		client, err := makeHTTPClient(settings)
		if err != nil {
			return s3Client{}, "", err
		}
		opts.Config.HTTPClient = client
	}

	switch conf.auth {
	case "", AuthParamSpecified:
		opts.Config.WithCredentials(credentials.NewStaticCredentials(conf.accessKey, conf.secret, conf.tempToken))
	case AuthParamImplicit:
		opts.SharedConfigState = session.SharedConfigEnable
	}

	// TODO(yevgeniy): Revisit retry logic.  Retrying 10 times seems arbitrary.
	opts.Config.MaxRetries = aws.Int(10)

	opts.Config.CredentialsChainVerboseErrors = aws.Bool(true)

	if conf.verbose {
		opts.Config.LogLevel = aws.LogLevel(aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors)
	}

	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return s3Client{}, "", errors.Wrap(err, "new aws session")
	}

	region := conf.region
	if region == "" {
		if err := delayedRetry(ctx, func() error {
			region, err = s3manager.GetBucketRegion(ctx, sess, conf.bucket, "us-east-1")
			return nil
		}); err != nil {
			return s3Client{}, "", errors.Wrap(err, "could not find s3 bucket's region")
		}
	}
	sess.Config.Region = aws.String(region)

	c := s3.New(sess)
	return s3Client{client: c}, region, nil
}

func (s *s3Storage) getClient(ctx context.Context) (*s3.S3, error) {
	if s.cached != nil {
		return s.cached.client, nil
	}
	client, region, err := newClient(ctx, s.opts, s.settings)
	if err != nil {
		return nil, err
	}
	if s.opts.region == "" {
		s.opts.region = region
	}
	return client.client, nil
}

func (s *s3Storage) Conf() roachpb.ExternalStorage {
	return roachpb.ExternalStorage{
		Provider: roachpb.ExternalStorageProvider_S3,
		S3Config: s.conf,
	}
}

func (s *s3Storage) ExternalIOConf() base.ExternalIODirConfig {
	return s.ioConf
}

func (s *s3Storage) Settings() *cluster.Settings {
	return s.settings
}

func (s *s3Storage) WriteFile(ctx context.Context, basename string, content io.ReadSeeker) error {
	client, err := s.getClient(ctx)
	if err != nil {
		return err
	}
	err = contextutil.RunWithTimeout(ctx, "put s3 object",
		timeoutSetting.Get(&s.settings.SV),
		func(ctx context.Context) error {
			putObjectInput := s3.PutObjectInput{
				Bucket: s.bucket,
				Key:    aws.String(path.Join(s.prefix, basename)),
				Body:   content,
			}

			// If a server side encryption mode is provided in the URI, we must set
			// the header values to enable SSE before writing the file to the s3
			// bucket.
			if s.conf.ServerEncMode != "" {
				switch s.conf.ServerEncMode {
				case string(aes256Enc):
					putObjectInput.SetServerSideEncryption(s.conf.ServerEncMode)
				case string(kmsEnc):
					putObjectInput.SetServerSideEncryption(s.conf.ServerEncMode)
					putObjectInput.SetSSEKMSKeyId(s.conf.ServerKMSID)
				default:
					return errors.Newf("unsupported server encryption mode %s. "+
						"Supported values are `aws:kms` and `AES256`.", s.conf.ServerEncMode)
				}
			}
			_, err := client.PutObjectWithContext(ctx, &putObjectInput)
			return err
		})
	return errors.Wrap(err, "failed to put s3 object")
}

func (s *s3Storage) ReadFile(ctx context.Context, basename string) (io.ReadCloser, error) {
	// https://github.com/cockroachdb/cockroach/issues/23859
	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	out, err := client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: s.bucket,
		Key:    aws.String(path.Join(s.prefix, basename)),
	})
	if err != nil {
		if aerr := (awserr.Error)(nil); errors.As(err, &aerr) {
			switch aerr.Code() {
			// Relevant 404 errors reported by AWS.
			case s3.ErrCodeNoSuchBucket, s3.ErrCodeNoSuchKey:
				return nil, errors.Wrapf(ErrFileDoesNotExist, "s3 object does not exist: %s", err.Error())
			}
		}
		return nil, errors.Wrap(err, "failed to get s3 object")
	}
	return out.Body, nil
}

func (s *s3Storage) ListFiles(ctx context.Context, patternSuffix string) ([]string, error) {
	var fileList []string

	pattern := s.prefix
	if patternSuffix != "" {
		if containsGlob(s.prefix) {
			return nil, errors.New("prefix cannot contain globs pattern when passing an explicit pattern")
		}
		pattern = path.Join(pattern, patternSuffix)
	}
	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}

	var matchErr error
	err = client.ListObjectsPagesWithContext(
		ctx,
		&s3.ListObjectsInput{
			Bucket: s.bucket,
			Prefix: aws.String(getPrefixBeforeWildcard(s.prefix)),
		},
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			for _, fileObject := range page.Contents {
				matches, err := path.Match(pattern, *fileObject.Key)
				if err != nil {
					matchErr = err
					return false
				}
				if matches {
					if patternSuffix != "" {
						if !strings.HasPrefix(*fileObject.Key, s.prefix) {
							// TODO(dt): return a nice rel-path instead of erroring out.
							matchErr = errors.New("pattern matched file outside of path")
							return false
						}
						fileList = append(fileList, strings.TrimPrefix(strings.TrimPrefix(*fileObject.Key, s.prefix), "/"))
					} else {
						s3URL := url.URL{
							Scheme:   "s3",
							Host:     *s.bucket,
							Path:     *fileObject.Key,
							RawQuery: s3QueryParams(s.conf),
						}
						fileList = append(fileList, s3URL.String())
					}
				}
			}
			return !lastPage
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, `failed to list s3 bucket`)
	}
	if matchErr != nil {
		return nil, errors.Wrap(matchErr, `failed to list s3 bucket`)
	}

	return fileList, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}

func (s *s3Storage) List(ctx context.Context, prefix, delim string, fn cloud.ListingFn) error {
	dest := JoinPathPreservingTrailingSlash(s.prefix, prefix)

	client, err := s.getClient(ctx)
	if err != nil {
		return err
	}

	var fnErr error
	pageFn := func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for _, x := range page.CommonPrefixes {
			if fnErr = fn(strings.TrimPrefix(*x.Prefix, dest)); fnErr != nil {
				return false
			}
		}
		for _, fileObject := range page.Contents {
			if fnErr = fn(strings.TrimPrefix(*fileObject.Key, dest)); fnErr != nil {
				return false
			}
		}
		return true
	}

	if err := client.ListObjectsPagesWithContext(
		ctx, &s3.ListObjectsInput{Bucket: s.bucket, Prefix: aws.String(dest), Delimiter: nilIfEmpty(delim)}, pageFn,
	); err != nil {
		return errors.Wrap(err, `failed to list s3 bucket`)
	}

	return fnErr
}

func (s *s3Storage) Delete(ctx context.Context, basename string) error {
	client, err := s.getClient(ctx)
	if err != nil {
		return err
	}
	return contextutil.RunWithTimeout(ctx, "delete s3 object",
		timeoutSetting.Get(&s.settings.SV),
		func(ctx context.Context) error {
			_, err := client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
				Bucket: s.bucket,
				Key:    aws.String(path.Join(s.prefix, basename)),
			})
			return err
		})
}

func (s *s3Storage) Size(ctx context.Context, basename string) (int64, error) {
	client, err := s.getClient(ctx)
	if err != nil {
		return 0, err
	}
	var out *s3.HeadObjectOutput
	err = contextutil.RunWithTimeout(ctx, "get s3 object header",
		timeoutSetting.Get(&s.settings.SV),
		func(ctx context.Context) error {
			var err error
			out, err = client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
				Bucket: s.bucket,
				Key:    aws.String(path.Join(s.prefix, basename)),
			})
			return err
		})
	if err != nil {
		return 0, errors.Wrap(err, "failed to get s3 object headers")
	}
	return *out.ContentLength, nil
}

func (s *s3Storage) Close() error {
	return nil
}
