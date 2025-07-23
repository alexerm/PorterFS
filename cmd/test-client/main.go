package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	endpoint := flag.String("endpoint", "http://localhost:9000", "PorterFS endpoint URL")
	accessKey := flag.String("access-key", "porterfs", "S3 access key")
	secretKey := flag.String("secret-key", "porterfs", "S3 secret key")
	bucket := flag.String("bucket", "test-bucket", "Test bucket name")
	flag.Parse()

	fmt.Printf("Testing PorterFS connection to %s\n", *endpoint)
	fmt.Printf("Using bucket: %s\n", *bucket)
	fmt.Println(strings.Repeat("=", 50))

	config := &aws.Config{
		Endpoint:         aws.String(*endpoint),
		Region:           aws.String("us-east-1"),
		Credentials:      credentials.NewStaticCredentials(*accessKey, *secretKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	}

	// For HTTPS endpoints with self-signed certificates
	if strings.HasPrefix(*endpoint, "https://") {
		// Note: In production, you should properly verify SSL certificates
		// For testing with self-signed certs, we disable SSL verification
		config.DisableSSL = aws.Bool(false)
		// Create a custom HTTP client that skips SSL verification
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		config.HTTPClient = &http.Client{Transport: tr}
	}

	sess, err := session.NewSession(config)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	svc := s3.New(sess)

	// Test 1: List buckets
	fmt.Println("1. Testing ListBuckets...")
	listResult, err := svc.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		fmt.Printf("   ❌ FAILED: %v\n", err)
	} else {
		fmt.Printf("   ✅ SUCCESS: Found %d buckets\n", len(listResult.Buckets))
		for _, b := range listResult.Buckets {
			fmt.Printf("      - %s\n", *b.Name)
		}
	}

	// Test 2: Create bucket
	fmt.Printf("\n2. Testing CreateBucket (%s)...\n", *bucket)
	_, err = svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(*bucket),
	})
	if err != nil {
		if strings.Contains(err.Error(), "BucketAlreadyExists") || strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") {
			fmt.Printf("   ✅ SUCCESS: Bucket already exists\n")
		} else {
			fmt.Printf("   ❌ FAILED: %v\n", err)
		}
	} else {
		fmt.Printf("   ✅ SUCCESS: Bucket created\n")
	}

	// Test 3: Put object
	fmt.Println("\n3. Testing PutObject...")
	testContent := "Hello from PorterFS test client!"
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(*bucket),
		Key:    aws.String("test-file.txt"),
		Body:   bytes.NewReader([]byte(testContent)),
	})
	if err != nil {
		fmt.Printf("   ❌ FAILED: %v\n", err)
	} else {
		fmt.Printf("   ✅ SUCCESS: Object uploaded\n")
	}

	// Test 4: List objects
	fmt.Println("\n4. Testing ListObjectsV2...")
	listObjResult, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(*bucket),
	})
	if err != nil {
		fmt.Printf("   ❌ FAILED: %v\n", err)
	} else {
		fmt.Printf("   ✅ SUCCESS: Found %d objects\n", len(listObjResult.Contents))
		for _, obj := range listObjResult.Contents {
			fmt.Printf("      - %s (%d bytes)\n", *obj.Key, *obj.Size)
		}
	}

	// Test 5: Get object
	fmt.Println("\n5. Testing GetObject...")
	getResult, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(*bucket),
		Key:    aws.String("test-file.txt"),
	})
	if err != nil {
		fmt.Printf("   ❌ FAILED: %v\n", err)
	} else {
		buf := make([]byte, 1024)
		n, _ := getResult.Body.Read(buf)
		getResult.Body.Close()
		content := string(buf[:n])
		if content == testContent {
			fmt.Printf("   ✅ SUCCESS: Content matches (%d bytes)\n", n)
		} else {
			fmt.Printf("   ❌ FAILED: Content mismatch\n")
			fmt.Printf("      Expected: %s\n", testContent)
			fmt.Printf("      Got: %s\n", content)
		}
	}

	// Test 6: Head object
	fmt.Println("\n6. Testing HeadObject...")
	headResult, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(*bucket),
		Key:    aws.String("test-file.txt"),
	})
	if err != nil {
		fmt.Printf("   ❌ FAILED: %v\n", err)
	} else {
		fmt.Printf("   ✅ SUCCESS: Object metadata retrieved\n")
		fmt.Printf("      Size: %d bytes\n", *headResult.ContentLength)
		fmt.Printf("      ETag: %s\n", *headResult.ETag)
	}

	// Test 7: Delete object
	fmt.Println("\n7. Testing DeleteObject...")
	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(*bucket),
		Key:    aws.String("test-file.txt"),
	})
	if err != nil {
		fmt.Printf("   ❌ FAILED: %v\n", err)
	} else {
		fmt.Printf("   ✅ SUCCESS: Object deleted\n")
	}

	// Test 8: Verify deletion
	fmt.Println("\n8. Testing object deletion verification...")
	_, err = svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(*bucket),
		Key:    aws.String("test-file.txt"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "Not Found") {
			fmt.Printf("   ✅ SUCCESS: Object not found (correctly deleted)\n")
		} else {
			fmt.Printf("   ❌ FAILED: Unexpected error: %v\n", err)
		}
	} else {
		fmt.Printf("   ❌ FAILED: Object still exists after deletion\n")
	}

	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("Test completed!")
	fmt.Println("\nTo run this test client:")
	fmt.Printf("  go run ./cmd/test-client -endpoint=%s -bucket=%s\n", *endpoint, *bucket)
}
