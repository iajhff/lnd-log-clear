package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	dbPath := os.Args[1]
	
	// Parse flags
	var olderThan time.Duration
	var bucketsToDelete []string
	
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		
		if strings.HasPrefix(arg, "--older-than=") {
			durationStr := strings.TrimPrefix(arg, "--older-than=")
			dur, err := parseDuration(durationStr)
			if err != nil {
				fmt.Printf("Error: Invalid duration '%s'\n", durationStr)
				fmt.Println("Valid formats: 1d, 1w, 2w, 1m, 3m, 6m, 1y")
				os.Exit(1)
			}
			olderThan = dur
		} else {
			bucketsToDelete = append(bucketsToDelete, arg)
		}
	}

	if len(bucketsToDelete) == 0 {
		fmt.Println("Error: No buckets specified")
		printUsage()
		os.Exit(1)
	}

	// Open database
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if olderThan > 0 {
		fmt.Printf("Clearing entries older than %v...\n\n", olderThan)
		clearOldEntries(db, bucketsToDelete, olderThan)
	} else {
		fmt.Println("Clearing entire buckets...\n")
		clearBuckets(db, bucketsToDelete)
	}

	fmt.Println("\n✓ Database cleanup complete")
}

// clearBuckets deletes entire buckets
func clearBuckets(db *bolt.DB, buckets []string) {
	db.Update(func(tx *bolt.Tx) error {
		for _, bucketName := range buckets {
			bucket := tx.Bucket([]byte(bucketName))
			if bucket == nil {
				fmt.Printf("  ⚠ Bucket '%s' doesn't exist (already empty)\n", bucketName)
				continue
			}

			err := tx.DeleteBucket([]byte(bucketName))
			if err != nil {
				fmt.Printf("  ✗ Failed to delete bucket '%s': %v\n", bucketName, err)
				continue
			}

			fmt.Printf("  ✓ Deleted entire bucket: %s\n", bucketName)
		}
		return nil
	})
}

// clearOldEntries deletes only entries older than the specified duration
func clearOldEntries(db *bolt.DB, buckets []string, olderThan time.Duration) {
	cutoffTime := time.Now().Add(-olderThan)
	cutoffNano := cutoffTime.UnixNano()

	db.Update(func(tx *bolt.Tx) error {
		for _, bucketName := range buckets {
			bucket := tx.Bucket([]byte(bucketName))
			if bucket == nil {
				fmt.Printf("  ⚠ Bucket '%s' doesn't exist\n", bucketName)
				continue
			}

			switch bucketName {
			case "circuit-fwd-log":
				deleted := clearForwardingLog(bucket, cutoffNano)
				fmt.Printf("  ✓ Deleted %d forwarding events from '%s' (older than %v)\n", 
					deleted, bucketName, olderThan)

			case "closed-chan-bucket", "historical-chan-bucket":
				fmt.Printf("  ⚠ Time-based cleanup not supported for '%s' (would delete entire bucket)\n", bucketName)
				fmt.Printf("    Use without --older-than flag to delete all\n")

			default:
				fmt.Printf("  ⚠ Time-based cleanup not supported for '%s'\n", bucketName)
			}
		}
		return nil
	})
}

// clearForwardingLog deletes forwarding events older than cutoff timestamp
func clearForwardingLog(bucket *bolt.Bucket, cutoffNano int64) int {
	deleted := 0
	keysToDelete := [][]byte{}

	// Collect keys to delete
	bucket.ForEach(func(k, v []byte) error {
		if len(k) == 8 {
			// Key is timestamp in nanoseconds (8 bytes, big endian)
			timestamp := int64(binary.BigEndian.Uint64(k))
			if timestamp < cutoffNano {
				keysToDelete = append(keysToDelete, append([]byte{}, k...))
			}
		}
		return nil
	})

	// Delete collected keys
	for _, key := range keysToDelete {
		bucket.Delete(key)
		deleted++
	}

	return deleted
}

// parseDuration parses duration strings like "1w", "2w", "1m", "3m"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration")
	}

	numStr := s[:len(s)-1]
	unit := s[len(s)-1:]

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}

	switch unit {
	case "d":
		return time.Duration(num) * 24 * time.Hour, nil
	case "w":
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(num) * 30 * 24 * time.Hour, nil
	case "y":
		return time.Duration(num) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid unit '%s', use: d, w, m, y", unit)
	}
}

func printUsage() {
	fmt.Println("LND Database History Cleaner")
	fmt.Println("============================")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  go run clear_db.go <db_path> [--older-than=DURATION] <bucket1> [bucket2] ...")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  # Clear all forwarding logs")
	fmt.Println("  go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db circuit-fwd-log")
	fmt.Println("")
	fmt.Println("  # Clear forwarding logs older than 1 week")
	fmt.Println("  go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=1w circuit-fwd-log")
	fmt.Println("")
	fmt.Println("  # Clear logs older than 2 weeks")
	fmt.Println("  go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=2w circuit-fwd-log")
	fmt.Println("")
	fmt.Println("  # Clear logs older than 1 month")
	fmt.Println("  go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=1m circuit-fwd-log")
	fmt.Println("")
	fmt.Println("  # Clear multiple buckets")
	fmt.Println("  go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db circuit-fwd-log closed-chan-bucket")
	fmt.Println("")
	fmt.Println("Duration formats:")
	fmt.Println("  1d, 7d   = days")
	fmt.Println("  1w, 2w   = weeks")
	fmt.Println("  1m, 3m   = months (30 days)")
	fmt.Println("  1y       = year")
	fmt.Println("")
	fmt.Println("Safe buckets to delete:")
	fmt.Println("  ✅ circuit-fwd-log          (forwarding history)")
	fmt.Println("  ✅ closed-chan-bucket       (old channel summaries)")
	fmt.Println("  ✅ historical-chan-bucket   (old channel details)")
	fmt.Println("")
	fmt.Println("NEVER delete these:")
	fmt.Println("  ❌ open-chan-bucket         (YOUR ACTIVE CHANNELS)")
	fmt.Println("  ❌ revocation-log           (BREACH DETECTION)")
	fmt.Println("  ❌ fwd-packages             (IN-FLIGHT HTLCS)")
}
