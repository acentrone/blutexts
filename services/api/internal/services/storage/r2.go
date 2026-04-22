// Package storage provides a client for Cloudflare R2 (S3-compatible) object storage.
// Used for media attachments sent with messages.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type R2Client struct {
	s3        *s3.Client
	bucket    string
	publicURL string // e.g. https://pub-xxx.r2.dev or custom domain
}

// NewR2Client creates an R2 client from environment variables.
// Returns nil if R2 is not configured (media features will be disabled).
func NewR2Client() *R2Client {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("R2_BUCKET")
	publicURL := os.Getenv("R2_PUBLIC_URL")

	if accountID == "" || accessKey == "" || secretKey == "" || bucket == "" || publicURL == "" {
		return nil
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &R2Client{
		s3:        client,
		bucket:    bucket,
		publicURL: strings.TrimRight(publicURL, "/"),
	}
}

// Upload stores a file in R2 and returns its public URL.
// The key includes a random UUID to prevent collisions and obscure filenames.
func (c *R2Client) Upload(ctx context.Context, data []byte, contentType, filename string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("R2 client not configured")
	}

	ext := ""
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		ext = strings.ToLower(filename[idx:])
	}

	// Store under date-based prefix for easier manual cleanup
	key := fmt.Sprintf("media/%s/%s%s", time.Now().Format("2006/01/02"), uuid.New().String(), ext)

	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(c.bucket),
		Key:                aws.String(key),
		Body:               bytes.NewReader(data),
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String(fmt.Sprintf(`inline; filename="%s"`, url.PathEscape(filename))),
	})
	if err != nil {
		return "", fmt.Errorf("r2 upload: %w", err)
	}

	return c.publicURL + "/" + key, nil
}

// Download fetches an object from a URL and returns the bytes.
// Used by the device agent to download attachments before sending via AppleScript.
func Download(ctx context.Context, fileURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, "", err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 100<<20)) // 100MB max
	if err != nil {
		return nil, "", err
	}

	return data, resp.Header.Get("Content-Type"), nil
}
