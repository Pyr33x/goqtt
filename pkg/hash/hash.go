package hash

import (
	"github.com/pyr33x/goqtt/pkg/er"
	"golang.org/x/crypto/bcrypt"
)

func HashPasswd(passwd string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(passwd), cost)
	if err != nil {
		return "", &er.Err{
			Context: "Hash",
			Message: er.ErrHashFailed,
		}
	}

	return string(hash), nil
}

func VerifyPasswd(hash, passwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(passwd))
	return err == nil
}
