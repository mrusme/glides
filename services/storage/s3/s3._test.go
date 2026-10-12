package s3

import (
	"errors"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
)

func TestBucketObjectFile(t *testing.T) {
	st := new(S3)

	bucket, key, name, err := st.getBucketObjectFile("uploads/2026/10/a.png")
	if err != nil || bucket != "uploads" || key != "2026/10/a.png" || name != "a.png" {
		t.Errorf("getBucketObjectFile = %q %q %q %v", bucket, key, name, err)
	}

	for _, dest := range []string{"", "bucket", "bucket/", "bucket/../other/x", "bucket//x", "bucket/./x", "/x"} {
		if _, _, _, err = st.getBucketObjectFile(dest); !errors.Is(err, errs.ErrFilePathInvalid) {
			t.Errorf("getBucketObjectFile(%q) = %v, want %v", dest, err, errs.ErrFilePathInvalid)
		}
	}
}
