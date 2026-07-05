package storage

import (
	"zoc/src/internal/clients/gcs"
)

var client *gcs.Client

// Init sets up the GCS client. If creds are missing, GetClient returns nil
// and document creation will fail with a clear 503 rather than panicking.
func Init(saJSON, bucket string) error {
	c, err := gcs.NewClient(saJSON, bucket)
	if err != nil {
		return err
	}
	client = c
	return nil
}

func GetClient() *gcs.Client {
	return client
}
