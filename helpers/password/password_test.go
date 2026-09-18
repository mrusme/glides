package password

import (
	"fmt"
	"strings"
	"testing"
)

func TestHashAndCheck(t *testing.T) {
	encoded, err := Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}

	prefix := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$", HashMem, HashIter, HashParallelism)
	if !strings.HasPrefix(encoded, prefix) {
		t.Errorf("Hash = %q, want the prefix %q on every machine", encoded, prefix)
	}

	if match, params, err := Check(encoded, "correct horse"); err != nil || !match || params.Parallelism != HashParallelism {
		t.Errorf("Check with the password = %v, %+v, %v", match, params, err)
	}
	if match, _, err := Check(encoded, "wrong horse"); err != nil || match {
		t.Errorf("Check with another password = %v, %v", match, err)
	}

	again, _ := Hash("correct horse")
	if again == encoded {
		t.Error("two hashes of one password are equal")
	}
}

func TestCheckAcceptsOtherParameters(t *testing.T) {
	const eightLanes = "$argon2id$v=19$m=65536,t=3,p=8$"

	for _, encoded := range []string{
		"not a hash",
		"$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$a2V5",
		"$argon2id$v=18$m=65536,t=3,p=4$c2FsdA$a2V5",
		eightLanes + "no separator",
	} {
		if match, _, err := Check(encoded, "anything"); err == nil || match {
			t.Errorf("Check(%q) = %v, %v", encoded, match, err)
		}
	}

	if _, params, err := Check(eightLanes+"c2FsdHNhbHRzYWx0c2FsdA$a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2U", "anything"); err != nil || params.Parallelism != 8 {
		t.Errorf("a hash made with 8 lanes: %+v, %v", params, err)
	}
}
