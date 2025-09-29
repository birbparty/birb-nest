// Package storage provides cloud storage integration for instance archival
package storage

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// SpacesConfig contains configuration for Digital Ocean Spaces
type SpacesConfig struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// SpacesClient provides operations for Digital Ocean Spaces
type SpacesClient struct {
	client     *s3.S3
	bucket     string
	pathPrefix string
}

// NewSpacesClient creates a new Digital Ocean Spaces client
func NewSpacesClient(config SpacesConfig) (*SpacesClient, error) {
	// Configure for Digital Ocean Spaces
	sess, err := session.NewSession(&aws.Config{
		Endpoint:    aws.String(config.Endpoint), // e.g., "nyc3.digitaloceanspaces.com"
		Region:      aws.String(config.Region),
		Credentials: credentials.NewStaticCredentials(config.AccessKey, config.SecretKey, ""),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &SpacesClient{
		client:     s3.New(sess),
		bucket:     config.Bucket,
		pathPrefix: "instance-archives/",
	}, nil
}

// UploadArchive uploads an instance archive to Spaces
func (s *SpacesClient) UploadArchive(instanceID string, data io.Reader) (string, error) {
	// Create path with date-based directory structure
	key := fmt.Sprintf("%s%s/%s.jsonl", s.pathPrefix, time.Now().Format("2006-01-02"), instanceID)

	// Convert io.Reader to bytes first (AWS SDK requires io.ReadSeeker)
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, data); err != nil {
		return "", fmt.Errorf("failed to read data: %w", err)
	}

	_, err := s.client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(buf.Bytes()),
		// Set metadata
		Metadata: map[string]*string{
			"instance-id":  aws.String(instanceID),
			"archive-time": aws.String(time.Now().UTC().Format(time.RFC3339)),
		},
		// Set content type
		ContentType: aws.String("application/x-jsonlines"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload archive: %w", err)
	}

	return key, nil
}

// GetArchive retrieves an instance archive from Spaces
func (s *SpacesClient) GetArchive(archivePath string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(archivePath),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get archive: %w", err)
	}

	return result.Body, nil
}

// ListArchives lists archives for a specific date
func (s *SpacesClient) ListArchives(date time.Time) ([]*s3.Object, error) {
	prefix := fmt.Sprintf("%s%s/", s.pathPrefix, date.Format("2006-01-02"))

	result, err := s.client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list archives: %w", err)
	}

	return result.Contents, nil
}

// DeleteArchive deletes an archive from Spaces
func (s *SpacesClient) DeleteArchive(archivePath string) error {
	_, err := s.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(archivePath),
	})
	if err != nil {
		return fmt.Errorf("failed to delete archive: %w", err)
	}

	return nil
}
