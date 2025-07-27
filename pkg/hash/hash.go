package hash

import (
	"golang.org/x/crypto/bcrypt"
)

func VerifyPasswd(hash, passwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(passwd))
	return err == nil
}
